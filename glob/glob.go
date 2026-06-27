package glob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
	"github.com/mattsp1290/eino-tools/result"
)

// Name is the model-facing tool name.
const Name = "glob"

const (
	DefaultLimit = 1000
	MaxLimit     = 5000
)

// Args is the parsed input shape for glob.
type Args struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// Result is the model-facing envelope returned by glob.
type Result struct {
	Outcome   result.Outcome  `json:"outcome"`
	Paths     []PathEntry     `json:"paths"`
	Count     int             `json:"count"`
	Truncated bool            `json:"truncated,omitempty"`
	Error     *ResultError    `json:"error,omitempty"`
	RawJSON   json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes Result and preserves the original object in RawJSON.
func (r *Result) UnmarshalJSON(raw []byte) error {
	type resultEnvelope Result
	var decoded resultEnvelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*r = Result(decoded)
	r.RawJSON = append(r.RawJSON[:0], raw...)
	return nil
}

// PathEntry is one matched workspace-relative path.
type PathEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir,omitempty"`
}

// ResultError is the structured failure envelope nested inside Result.
type ResultError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

const (
	ErrCategoryValidation   = "validation"
	ErrCategoryPathEscape   = "path_escape"
	ErrCategoryNotFound     = "not_found"
	ErrCategoryNotDirectory = "not_directory"
	ErrCategoryIO           = "io"
	ErrCategoryCanceled     = "canceled"
	ErrCategoryUnknown      = "unknown"
)

// IsRetryable reports whether the agent loop should retry this call.
func (r Result) IsRetryable() bool {
	if r.Outcome == result.OutcomeSucceeded {
		return false
	}
	if r.Error == nil {
		return false
	}
	switch r.Error.Category {
	case ErrCategoryIO, ErrCategoryUnknown:
		return true
	default:
		return false
	}
}

// Tool discovers paths under a configured workspace.
type Tool struct {
	workspacePath string
}

// New constructs a Tool from an absolute, resolvable workspace path.
func New(workspacePath string) (*Tool, error) {
	if workspacePath == "" {
		return nil, errors.New("glob: workspace path is required")
	}
	if !filepath.IsAbs(workspacePath) {
		return nil, fmt.Errorf("glob: workspace path must be absolute, got %q", workspacePath)
	}
	resolved, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("glob: resolve workspace path %q: %w", workspacePath, err)
	}
	return &Tool{workspacePath: resolved}, nil
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "pattern": {
      "type": "string",
      "minLength": 1,
      "description": "Doublestar glob pattern to match against paths under the search root, e.g. \"*.go\" or \"**/*_test.go\"."
    },
    "path": {
      "type": "string",
      "description": "Workspace-relative directory to search. Omit or use \".\" for the workspace root."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 5000,
      "description": "Maximum number of paths to return. Default 1000; hard cap 5000."
    }
  },
  "required": ["pattern"]
}`

// Schema returns a fresh JSON Schema copy for glob arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

// Run walks the configured workspace and returns matched paths.
func (t *Tool) Run(ctx context.Context, args Args) Result {
	if t == nil {
		return failedResult(ErrCategoryValidation, "glob tool is not configured for this run")
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return failedResult(ErrCategoryValidation, "pattern is required and must be non-empty")
	}
	if perr := validatePattern(args.Pattern); perr != nil {
		return failedResult(perr.category, perr.message)
	}
	limit := args.Limit
	if limit == 0 {
		limit = DefaultLimit
	}
	if limit < 0 {
		return failedResult(ErrCategoryValidation, fmt.Sprintf("limit must be non-negative, got %d", args.Limit))
	}
	if limit > MaxLimit {
		return failedResult(ErrCategoryValidation, fmt.Sprintf("limit %d exceeds maximum %d", args.Limit, MaxLimit))
	}
	if err := ctx.Err(); err != nil {
		return failedResult(contextErrCategory(err), err.Error())
	}

	rel := args.Path
	if rel == "" {
		rel = "."
	}
	rel = filepath.Clean(rel)
	var root string
	if rel == "." {
		root = t.workspacePath
	} else {
		var perr *pathErr
		root, perr = resolveExisting(t.workspacePath, rel)
		if perr != nil {
			return failedResult(perr.category, perr.message)
		}
	}
	info, err := os.Stat(root)
	if err != nil {
		return failedResult(ErrCategoryIO, fmt.Sprintf("stat %q: %v", rel, err))
	}
	if !info.IsDir() {
		return failedResult(ErrCategoryNotDirectory, fmt.Sprintf("path %q is not a directory; glob expects a directory", rel))
	}

	pattern := filepath.ToSlash(args.Pattern)
	paths := make([]PathEntry, 0, min(limit, DefaultLimit))
	truncated := false
	walkErr := filepath.WalkDir(root, func(p string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == root {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() && isVCSDir(d.Name()) {
			return filepath.SkipDir
		}

		isDir := d.IsDir()
		if d.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(p)
			if err != nil {
				return err
			}
			if !isDescendant(t.workspacePath, target, true) {
				return newPathErr(ErrCategoryPathEscape, "path %q resolves outside workspace", displayPath(t.workspacePath, p))
			}
			if targetInfo, err := os.Stat(target); err == nil {
				isDir = targetInfo.IsDir()
			}
		}

		searchRel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		searchRel = filepath.ToSlash(searchRel)
		matched, err := doublestar.Match(pattern, searchRel)
		if err != nil {
			return newPathErr(ErrCategoryValidation, "invalid glob pattern %q: %v", args.Pattern, err)
		}
		if !matched {
			return nil
		}

		workspaceRel, err := filepath.Rel(t.workspacePath, p)
		if err != nil {
			return err
		}
		paths = append(paths, PathEntry{Path: filepath.ToSlash(workspaceRel), IsDir: isDir})
		if len(paths) > limit {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, filepath.SkipAll) {
		if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
			return failedResult(ErrCategoryCanceled, walkErr.Error())
		}
		var perr *pathErr
		if errors.As(walkErr, &perr) {
			return failedResult(perr.category, perr.message)
		}
		return failedResult(ErrCategoryIO, fmt.Sprintf("walk %q: %v", rel, walkErr))
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Path < paths[j].Path
	})
	if len(paths) > limit {
		paths = paths[:limit]
	}
	if paths == nil {
		paths = []PathEntry{}
	}
	return Result{
		Outcome:   result.OutcomeSucceeded,
		Paths:     paths,
		Count:     len(paths),
		Truncated: truncated,
	}
}

// Info returns the Eino ToolInfo for glob.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("glob: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Discover workspace-relative paths with doublestar glob semantics (*, ?, **). Hidden files are included by default; VCS internals are skipped. Results are sorted, capped, and returned with is_dir metadata.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("glob: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("glob: parse arguments: %w", err)
	}
	var args Args
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("glob: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("glob: marshal result: %w", err)
	}
	return string(out), nil
}

type pathErr struct {
	category string
	message  string
}

func (e *pathErr) Error() string { return e.message }

func newPathErr(category, format string, a ...any) *pathErr {
	return &pathErr{category: category, message: fmt.Sprintf(format, a...)}
}

func validatePattern(pattern string) *pathErr {
	if strings.ContainsRune(pattern, 0) {
		return newPathErr(ErrCategoryValidation, "pattern contains NUL byte")
	}
	slash := filepath.ToSlash(pattern)
	if path.IsAbs(slash) {
		return newPathErr(ErrCategoryValidation, "pattern must be relative, got absolute %q", pattern)
	}
	for _, part := range strings.Split(slash, "/") {
		if part == ".." {
			return newPathErr(ErrCategoryPathEscape, "pattern %q contains parent traversal", pattern)
		}
	}
	if _, err := doublestar.Match(slash, "probe"); err != nil {
		return newPathErr(ErrCategoryValidation, "invalid glob pattern %q: %v", pattern, err)
	}
	return nil
}

func resolveExisting(workspacePath, rel string) (string, *pathErr) {
	if rel == "" {
		return "", newPathErr(ErrCategoryValidation, "path is required")
	}
	if filepath.IsAbs(rel) {
		return "", newPathErr(ErrCategoryValidation, "path must be workspace-relative, got absolute %q", rel)
	}
	if strings.ContainsRune(rel, 0) {
		return "", newPathErr(ErrCategoryValidation, "path contains NUL byte")
	}
	candidate := filepath.Join(workspacePath, rel)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newPathErr(ErrCategoryNotFound, "path does not exist: %s", rel)
		}
		return "", newPathErr(ErrCategoryUnknown, "resolve path %q: %v", rel, err)
	}
	if !isDescendant(workspacePath, resolved, true) {
		return "", newPathErr(ErrCategoryPathEscape, "path %q resolves outside workspace", rel)
	}
	return resolved, nil
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

func isVCSDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", ".jj":
		return true
	default:
		return false
	}
}

func displayPath(workspacePath, p string) string {
	rel, err := filepath.Rel(workspacePath, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

func rejectDuplicateTopLevelKeys(raw []byte) error {
	return jsoncompat.RejectDuplicateTopLevelKeys(raw)
}

func failedResult(category, message string) Result {
	return Result{
		Outcome: result.OutcomeFailed,
		Paths:   []PathEntry{},
		Count:   0,
		Error:   &ResultError{Category: category, Message: message},
	}
}

func contextErrCategory(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrCategoryCanceled
	}
	return ErrCategoryUnknown
}

var (
	_ interface {
		Run(context.Context, Args) Result
	} = (*Tool)(nil)
	_ tool.InvokableTool = (*Tool)(nil)
)
