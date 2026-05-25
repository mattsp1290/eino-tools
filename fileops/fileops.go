package fileops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/result"
)

// Tool names. Stable: dashboards, metric labels, and prompts may pin on these
// literals. Renaming is a coordinated breaking change.
const (
	NameRead  = "file_read"
	NameWrite = "file_write"
	NameEdit  = "file_edit"
	NameList  = "file_list"
)

const (
	// MaxOutputBytes caps file_read output and file_write/file_edit input.
	MaxOutputBytes = 256 * 1024

	// MaxListEntries caps file_list output.
	MaxListEntries = 5000
)

// BaseResult is embedded by every per-tool result. Outcome is always
// populated; Error is non-nil iff Outcome != result.OutcomeSucceeded.
//
// RawJSON captures the original JSON object when a result is decoded. It is
// not emitted when marshaling. This lets consumers inspect unknown top-level
// fields added by future versions without forcing this package to publish a
// universal result envelope.
type BaseResult struct {
	Outcome result.Outcome  `json:"outcome"`
	Error   *ResultError    `json:"error,omitempty"`
	RawJSON json.RawMessage `json:"-"`
}

// IsRetryable reports whether the agent loop should attempt the same file_*
// call again after this result.
func (b BaseResult) IsRetryable() bool {
	if b.Outcome == result.OutcomeSucceeded {
		return false
	}
	if b.Error == nil {
		return false
	}
	switch b.Error.Category {
	case ErrCategoryIO, ErrCategoryUnknown:
		return true
	default:
		return false
	}
}

// ResultError is the structured failure envelope. Category is the
// discriminator the model branches on; Message is human-readable.
type ResultError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

// Tool-level error categories. Strings are model-facing.
const (
	ErrCategoryValidation      = "validation"
	ErrCategoryPathEscape      = "path_escape"
	ErrCategoryNotFound        = "not_found"
	ErrCategoryIsDirectory     = "is_directory"
	ErrCategoryNotDirectory    = "not_directory"
	ErrCategoryAnchorNotFound  = "anchor_not_found"
	ErrCategoryAnchorAmbiguous = "anchor_ambiguous"
	ErrCategoryTooLarge        = "too_large"
	ErrCategoryIO              = "io"
	ErrCategoryCanceled        = "canceled"
	ErrCategoryUnknown         = "unknown"
)

type pathErr struct {
	category string
	message  string
}

func (e *pathErr) Error() string { return e.message }

func newPathErr(category, format string, a ...any) *pathErr {
	return &pathErr{category: category, message: fmt.Sprintf(format, a...)}
}

func validateRelPath(rel string) *pathErr {
	if rel == "" {
		return newPathErr(ErrCategoryValidation, "path is required")
	}
	if filepath.IsAbs(rel) {
		return newPathErr(ErrCategoryValidation,
			"path must be workspace-relative, got absolute %q", rel)
	}
	if strings.ContainsRune(rel, 0) {
		return newPathErr(ErrCategoryValidation, "path contains NUL byte")
	}
	return nil
}

func resolveExisting(workspacePath, rel string, allowRoot bool) (string, *pathErr) {
	if perr := validateRelPath(rel); perr != nil {
		return "", perr
	}
	candidate := filepath.Join(workspacePath, rel)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newPathErr(ErrCategoryNotFound, "path does not exist: %s", rel)
		}
		return "", newPathErr(ErrCategoryIO, "resolve path %q: %v", rel, err)
	}
	if !isDescendant(workspacePath, resolved, allowRoot) {
		return "", newPathErr(ErrCategoryPathEscape,
			"path %q resolves outside workspace", rel)
	}
	return resolved, nil
}

func resolveWritable(workspacePath, rel string, createDirs bool) (string, *pathErr) {
	if perr := validateRelPath(rel); perr != nil {
		return "", perr
	}
	candidate := filepath.Clean(filepath.Join(workspacePath, rel))

	if !syntacticDescendant(workspacePath, candidate) {
		return "", newPathErr(ErrCategoryPathEscape,
			"path %q resolves outside workspace", rel)
	}

	parent := filepath.Dir(candidate)
	parentResolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !createDirs {
				return "", newPathErr(ErrCategoryNotFound,
					"parent directory does not exist: %s (set create_dirs=true to mkdir -p)",
					filepath.Dir(rel))
			}
			ancestor, ferr := firstExistingAncestor(parent)
			if ferr != nil {
				return "", newPathErr(ErrCategoryIO, "locate ancestor of %q: %v", rel, ferr)
			}
			ancestorResolved, ferr := filepath.EvalSymlinks(ancestor)
			if ferr != nil {
				return "", newPathErr(ErrCategoryIO, "resolve ancestor of %q: %v", rel, ferr)
			}
			if !isDescendant(workspacePath, ancestorResolved, true) {
				return "", newPathErr(ErrCategoryPathEscape,
					"ancestor of %q resolves outside workspace", rel)
			}
			if mkErr := os.MkdirAll(parent, 0o750); mkErr != nil {
				return "", newPathErr(ErrCategoryIO,
					"create parent directories for %q: %v", rel, mkErr)
			}
			parentResolved, err = filepath.EvalSymlinks(parent)
			if err != nil {
				return "", newPathErr(ErrCategoryIO, "resolve parent after mkdir: %v", err)
			}
		} else {
			return "", newPathErr(ErrCategoryIO, "resolve parent of %q: %v", rel, err)
		}
	}
	if !isDescendant(workspacePath, parentResolved, true) {
		return "", newPathErr(ErrCategoryPathEscape,
			"parent of %q resolves outside workspace", rel)
	}
	if leafResolved, lerr := filepath.EvalSymlinks(candidate); lerr == nil {
		if !isDescendant(workspacePath, leafResolved, true) {
			return "", newPathErr(ErrCategoryPathEscape,
				"existing leaf at %q is a symlink resolving outside workspace", rel)
		}
	}
	return candidate, nil
}

func firstExistingAncestor(p string) (string, error) {
	for {
		if _, err := os.Lstat(p); err == nil {
			return p, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p, nil
		}
		p = parent
	}
}

func syntacticDescendant(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if filepath.IsAbs(rel) {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func isDescendant(parent, resolved string, allowRoot bool) bool {
	parent = filepath.Clean(parent)
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(parent, resolved)
	if err != nil {
		return false
	}
	if rel == "." {
		return allowRoot
	}
	if filepath.IsAbs(rel) {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func rejectDuplicateTopLevelKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil //nolint:nilerr
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil
	}
	seen := make(map[string]struct{})
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil //nolint:nilerr
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil
		}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate top-level key %q", key)
		}
		seen[key] = struct{}{}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil //nolint:nilerr
		}
	}
	return nil
}

func validateWorkspacePath(workspacePath string) error {
	if workspacePath == "" {
		return errors.New("fileops: workspace path is required")
	}
	if !filepath.IsAbs(workspacePath) {
		return fmt.Errorf("fileops: workspace path must be absolute, got %q", workspacePath)
	}
	return nil
}

func canonicalizeWorkspace(workspacePath string) string {
	resolved, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return workspacePath
	}
	return resolved
}

func failed(category, message string) BaseResult {
	return BaseResult{
		Outcome: result.OutcomeFailed,
		Error: &ResultError{
			Category: category,
			Message:  message,
		},
	}
}

func failedFromPathErr(perr *pathErr) BaseResult {
	return failed(perr.category, perr.message)
}

func contextErrCategory(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrCategoryCanceled
	}
	return ErrCategoryUnknown
}

func buildToolInfo(name, desc string, schemaJSON []byte) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal(schemaJSON, js); err != nil {
		return nil, fmt.Errorf("%s: parse tool schema: %w", name, err)
	}
	return &schema.ToolInfo{
		Name:        name,
		Desc:        desc,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}
