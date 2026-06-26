package applypatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
	"github.com/mattsp1290/eino-tools/result"
)

// Name is the model-facing tool name.
const Name = "apply_patch"

const (
	MaxPatchBytes = 1024 * 1024
	addFileMode   = 0o644
)

// Args is the parsed input shape for apply_patch.
type Args struct {
	PatchText string `json:"patch_text"`
}

// Result is the model-facing envelope returned by apply_patch.
type Result struct {
	Outcome       result.Outcome  `json:"outcome"`
	Files         []FileResult    `json:"files"`
	Partial       bool            `json:"partial,omitempty"`
	DiffTruncated bool            `json:"diff_truncated,omitempty"`
	Error         *ResultError    `json:"error,omitempty"`
	RawJSON       json.RawMessage `json:"-"`
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

// FileResult summarizes one file operation.
type FileResult struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	NewPath   string `json:"new_path,omitempty"`
	Status    string `json:"status"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
}

// ResultError is the structured failure envelope nested inside Result.
type ResultError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

const (
	ErrCategoryValidation  = "validation"
	ErrCategoryPathEscape  = "path_escape"
	ErrCategoryNotFound    = "not_found"
	ErrCategoryIsDirectory = "is_directory"
	ErrCategoryTooLarge    = "too_large"
	ErrCategoryUnsupported = "unsupported"
	ErrCategoryConflict    = "conflict"
	ErrCategoryBinary      = "binary"
	ErrCategoryIO          = "io"
	ErrCategoryCanceled    = "canceled"
	ErrCategoryUnknown     = "unknown"
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

// Tool applies structured patches under a configured workspace.
type Tool struct {
	workspacePath string
}

// New constructs a Tool from an absolute, resolvable workspace path.
func New(workspacePath string) (*Tool, error) {
	if workspacePath == "" {
		return nil, errors.New("applypatch: workspace path is required")
	}
	if !filepath.IsAbs(workspacePath) {
		return nil, fmt.Errorf("applypatch: workspace path must be absolute, got %q", workspacePath)
	}
	resolved, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("applypatch: resolve workspace path %q: %w", workspacePath, err)
	}
	return &Tool{workspacePath: resolved}, nil
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "patch_text": {
      "type": "string",
      "minLength": 1,
      "description": "Patch text using the *** Begin Patch / *** End Patch grammar. Cap is 1 MiB."
    }
  },
  "required": ["patch_text"]
}`

// Schema returns a fresh JSON Schema copy for apply_patch arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

// Run parses, preflights, and applies a patch.
func (t *Tool) Run(ctx context.Context, args Args) Result {
	if t == nil {
		return failedResult(ErrCategoryValidation, "apply_patch tool is not configured for this run", nil)
	}
	if err := ctx.Err(); err != nil {
		return failedResult(contextErrCategory(err), err.Error(), nil)
	}
	if strings.TrimSpace(args.PatchText) == "" {
		return failedResult(ErrCategoryValidation, "patch_text is required and must be non-empty", nil)
	}
	if len(args.PatchText) > MaxPatchBytes {
		return failedResult(ErrCategoryTooLarge,
			fmt.Sprintf("patch_text is %d bytes; max %d", len(args.PatchText), MaxPatchBytes), nil)
	}

	parsed, err := parsePatch(args.PatchText)
	if err != nil {
		return failedResult(ErrCategoryValidation, err.Error(), nil)
	}
	planned, fileResults, rerr := t.preflight(ctx, parsed)
	if rerr != nil {
		return failedResult(rerr.Category, rerr.Message, fileResults)
	}
	return t.commit(ctx, planned, fileResults)
}

// Info returns the Eino ToolInfo for apply_patch.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("applypatch: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Apply a multi-file structured patch under the workspace. Supports add, update, delete, and move. Preflights every target before writing and returns per-file operation summaries; partial=true is reserved for commit-time failures after preflight.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("applypatch: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("applypatch: parse arguments: %w", err)
	}
	var args Args
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("applypatch: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("applypatch: marshal result: %w", err)
	}
	return string(out), nil
}

type parsedPatch struct {
	ops []fileOp
}

type fileOp struct {
	op        string
	path      string
	newPath   string
	addLines  []string
	hunks     []hunk
	additions int
	deletions int
}

type hunk struct {
	oldLines  []string
	newLines  []string
	additions int
	deletions int
}

func parsePatch(patchText string) (parsedPatch, error) {
	normalized := normalizePatchText(patchText)
	lines := strings.Split(normalized, "\n")
	if len(lines) < 2 || lines[0] != "*** Begin Patch" {
		return parsedPatch{}, errors.New("patch must start with *** Begin Patch")
	}
	end := len(lines) - 1
	for end > 0 && lines[end] == "" {
		end--
	}
	if lines[end] != "*** End Patch" {
		return parsedPatch{}, errors.New("patch must end with *** End Patch")
	}
	lines = lines[1:end]
	parsed := parsedPatch{ops: make([]fileOp, 0)}
	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			op := fileOp{op: "add", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Add File: "))}
			i++
			for i < len(lines) && !isOperationLine(lines[i]) {
				if !strings.HasPrefix(lines[i], "+") {
					return parsedPatch{}, fmt.Errorf("add file %q expects + lines, got %q", op.path, lines[i])
				}
				op.addLines = append(op.addLines, strings.TrimPrefix(lines[i], "+"))
				op.additions++
				i++
			}
			if len(op.addLines) == 0 {
				return parsedPatch{}, fmt.Errorf("add file %q has no content lines", op.path)
			}
			parsed.ops = append(parsed.ops, op)
		case strings.HasPrefix(line, "*** Update File: "):
			op := fileOp{op: "update", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Update File: "))}
			i++
			if i < len(lines) && strings.HasPrefix(lines[i], "*** Move to: ") {
				op.newPath = strings.TrimSpace(strings.TrimPrefix(lines[i], "*** Move to: "))
				i++
			}
			for i < len(lines) && !isOperationLine(lines[i]) {
				if !strings.HasPrefix(lines[i], "@@") {
					return parsedPatch{}, fmt.Errorf("update file %q expects @@ hunk, got %q", op.path, lines[i])
				}
				i++
				var h hunk
				for i < len(lines) && !isOperationLine(lines[i]) && !strings.HasPrefix(lines[i], "@@") {
					if lines[i] == "" {
						return parsedPatch{}, fmt.Errorf("hunk for %q has an empty patch line without a prefix", op.path)
					}
					prefix := lines[i][0]
					text := lines[i][1:]
					switch prefix {
					case ' ':
						h.oldLines = append(h.oldLines, text)
						h.newLines = append(h.newLines, text)
					case '-':
						h.oldLines = append(h.oldLines, text)
						h.deletions++
						op.deletions++
					case '+':
						h.newLines = append(h.newLines, text)
						h.additions++
						op.additions++
					default:
						return parsedPatch{}, fmt.Errorf("hunk for %q has invalid line prefix %q", op.path, string(prefix))
					}
					i++
				}
				if len(h.oldLines) == 0 {
					return parsedPatch{}, fmt.Errorf("hunk for %q has no context or removed lines to match", op.path)
				}
				op.hunks = append(op.hunks, h)
			}
			if op.newPath == "" && len(op.hunks) == 0 {
				return parsedPatch{}, fmt.Errorf("update file %q has no hunks", op.path)
			}
			parsed.ops = append(parsed.ops, op)
		case strings.HasPrefix(line, "*** Delete File: "):
			op := fileOp{op: "delete", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File: "))}
			parsed.ops = append(parsed.ops, op)
			i++
		default:
			return parsedPatch{}, fmt.Errorf("unexpected patch line %q", line)
		}
	}
	if len(parsed.ops) == 0 {
		return parsedPatch{}, errors.New("patch contains no file operations")
	}
	return parsed, nil
}

func isOperationLine(line string) bool {
	return strings.HasPrefix(line, "*** Add File: ") ||
		strings.HasPrefix(line, "*** Update File: ") ||
		strings.HasPrefix(line, "*** Delete File: ")
}

type plannedChange struct {
	op        fileOp
	srcPath   string
	dstPath   string
	content   []byte
	mode      os.FileMode
	removeSrc bool
}

func (t *Tool) preflight(ctx context.Context, parsed parsedPatch) ([]plannedChange, []FileResult, *ResultError) {
	seen := make(map[string]string)
	planned := make([]plannedChange, 0, len(parsed.ops))
	fileResults := make([]FileResult, 0, len(parsed.ops))
	for _, op := range parsed.ops {
		if err := ctx.Err(); err != nil {
			return nil, fileResults, &ResultError{Category: contextErrCategory(err), Message: err.Error()}
		}
		if op.path == "" {
			return nil, fileResults, &ResultError{Category: ErrCategoryValidation, Message: "file operation path is required"}
		}
		if rerr := markTarget(seen, op.path, op.op); rerr != nil {
			return nil, fileResults, rerr
		}
		if op.newPath != "" {
			if rerr := markTarget(seen, op.newPath, op.op); rerr != nil {
				return nil, fileResults, rerr
			}
		}
		fr := FileResult{
			Operation: op.op,
			Path:      op.path,
			NewPath:   op.newPath,
			Status:    "preflighted",
			Additions: op.additions,
			Deletions: op.deletions,
		}
		fileResults = append(fileResults, fr)

		switch op.op {
		case "add":
			dst, perr := resolveWritable(t.workspacePath, op.path)
			if perr != nil {
				return nil, fileResults, &ResultError{Category: perr.category, Message: perr.message}
			}
			if _, err := os.Lstat(dst); err == nil {
				return nil, fileResults, &ResultError{Category: ErrCategoryValidation, Message: fmt.Sprintf("add target already exists: %s", op.path)}
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, fileResults, &ResultError{Category: ErrCategoryIO, Message: fmt.Sprintf("stat add target %q: %v", op.path, err)}
			}
			planned = append(planned, plannedChange{
				op:      op,
				dstPath: dst,
				content: []byte(linesToText(op.addLines)),
				mode:    addFileMode,
			})
		case "update":
			src, info, data, lineEnding, rerr := t.readTextFile(op.path)
			if rerr != nil {
				return nil, fileResults, rerr
			}
			content := string(data)
			for _, h := range op.hunks {
				oldText := linesToText(h.oldLines)
				newText := linesToText(h.newLines)
				count := strings.Count(content, oldText)
				switch count {
				case 0:
					return nil, fileResults, &ResultError{Category: ErrCategoryConflict, Message: fmt.Sprintf("context did not match in %s", op.path)}
				case 1:
					content = strings.Replace(content, oldText, newText, 1)
				default:
					return nil, fileResults, &ResultError{Category: ErrCategoryConflict, Message: fmt.Sprintf("context matched %d times in %s; add more context", count, op.path)}
				}
			}
			dst := src
			removeSrc := false
			if op.newPath != "" {
				var perr *pathErr
				dst, perr = resolveWritable(t.workspacePath, op.newPath)
				if perr != nil {
					return nil, fileResults, &ResultError{Category: perr.category, Message: perr.message}
				}
				if _, err := os.Lstat(dst); err == nil {
					return nil, fileResults, &ResultError{Category: ErrCategoryValidation, Message: fmt.Sprintf("move target already exists: %s", op.newPath)}
				} else if !errors.Is(err, os.ErrNotExist) {
					return nil, fileResults, &ResultError{Category: ErrCategoryIO, Message: fmt.Sprintf("stat move target %q: %v", op.newPath, err)}
				}
				removeSrc = true
			}
			planned = append(planned, plannedChange{
				op:        op,
				srcPath:   src,
				dstPath:   dst,
				content:   []byte(restoreLineEndings(content, lineEnding)),
				mode:      info.Mode().Perm(),
				removeSrc: removeSrc,
			})
		case "delete":
			src, info, rerr := t.statExistingFile(op.path)
			if rerr != nil {
				return nil, fileResults, rerr
			}
			planned = append(planned, plannedChange{op: op, srcPath: src, mode: info.Mode().Perm(), removeSrc: true})
		default:
			return nil, fileResults, &ResultError{Category: ErrCategoryUnsupported, Message: fmt.Sprintf("unsupported operation %q", op.op)}
		}
	}
	return planned, fileResults, nil
}

func (t *Tool) commit(ctx context.Context, planned []plannedChange, fileResults []FileResult) Result {
	partial := false
	for i, change := range planned {
		if err := ctx.Err(); err != nil {
			return Result{
				Outcome: result.OutcomeFailed,
				Files:   fileResults,
				Partial: partial,
				Error:   &ResultError{Category: contextErrCategory(err), Message: err.Error()},
			}
		}
		switch change.op.op {
		case "add", "update":
			if err := os.MkdirAll(filepath.Dir(change.dstPath), 0o750); err != nil {
				return commitFailed(fileResults, partial, i, ErrCategoryIO, fmt.Sprintf("create parent directories for %q: %v", change.op.path, err))
			}
			if err := atomicWrite(change.dstPath, change.content, change.mode); err != nil {
				return commitFailed(fileResults, partial, i, ErrCategoryIO, fmt.Sprintf("write %q: %v", resultPath(change.op), err))
			}
			if change.removeSrc {
				if err := os.Remove(change.srcPath); err != nil {
					fileResults[i].Status = "partial"
					return Result{
						Outcome: result.OutcomeFailed,
						Files:   fileResults,
						Partial: true,
						Error:   &ResultError{Category: ErrCategoryIO, Message: fmt.Sprintf("remove moved source %q: %v", change.op.path, err)},
					}
				}
			}
		case "delete":
			if err := os.Remove(change.srcPath); err != nil {
				return commitFailed(fileResults, partial, i, ErrCategoryIO, fmt.Sprintf("delete %q: %v", change.op.path, err))
			}
		}
		fileResults[i].Status = "applied"
		partial = true
	}
	return Result{
		Outcome: result.OutcomeSucceeded,
		Files:   fileResults,
	}
}

func commitFailed(fileResults []FileResult, partial bool, index int, category, message string) Result {
	if index >= 0 && index < len(fileResults) {
		fileResults[index].Status = "failed"
	}
	return Result{
		Outcome: result.OutcomeFailed,
		Files:   fileResults,
		Partial: partial,
		Error:   &ResultError{Category: category, Message: message},
	}
}

func (t *Tool) readTextFile(rel string) (string, os.FileInfo, []byte, string, *ResultError) {
	resolved, info, rerr := t.statExistingFile(rel)
	if rerr != nil {
		return "", nil, nil, "", rerr
	}
	data, err := os.ReadFile(resolved) //nolint:gosec // path verified by resolveExisting
	if err != nil {
		return "", nil, nil, "", &ResultError{Category: ErrCategoryIO, Message: fmt.Sprintf("read %q: %v", rel, err)}
	}
	if containsNUL(data) || !utf8.Valid(data) {
		return "", nil, nil, "", &ResultError{Category: ErrCategoryBinary, Message: fmt.Sprintf("file %q appears to be binary or non-UTF-8 text", rel)}
	}
	normalized, lineEnding := normalizeFileText(string(data))
	return resolved, info, []byte(normalized), lineEnding, nil
}

func (t *Tool) statExistingFile(rel string) (string, os.FileInfo, *ResultError) {
	resolved, perr := resolveExisting(t.workspacePath, rel)
	if perr != nil {
		return "", nil, &ResultError{Category: perr.category, Message: perr.message}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, &ResultError{Category: ErrCategoryIO, Message: fmt.Sprintf("stat %q: %v", rel, err)}
	}
	if info.IsDir() {
		return "", nil, &ResultError{Category: ErrCategoryIsDirectory, Message: fmt.Sprintf("path %q is a directory", rel)}
	}
	return resolved, info, nil
}

func markTarget(seen map[string]string, rel, op string) *ResultError {
	if rel == "" {
		return &ResultError{Category: ErrCategoryValidation, Message: "target path is required"}
	}
	cleaned := filepath.ToSlash(filepath.Clean(rel))
	if prior, ok := seen[cleaned]; ok {
		return &ResultError{Category: ErrCategoryValidation, Message: fmt.Sprintf("duplicate target path %q in %s and %s operations", rel, prior, op)}
	}
	seen[cleaned] = op
	return nil
}

func normalizePatchText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func normalizeFileText(s string) (string, string) {
	lineEnding := "\n"
	if strings.Contains(s, "\r\n") {
		lineEnding = "\r\n"
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, lineEnding
}

func restoreLineEndings(s, lineEnding string) string {
	if lineEnding == "\r\n" {
		return strings.ReplaceAll(s, "\n", "\r\n")
	}
	return s
}

func linesToText(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func resultPath(op fileOp) string {
	if op.newPath != "" {
		return op.newPath
	}
	return op.path
}

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
		return newPathErr(ErrCategoryValidation, "path must be workspace-relative, got absolute %q", rel)
	}
	if strings.ContainsRune(rel, 0) {
		return newPathErr(ErrCategoryValidation, "path contains NUL byte")
	}
	return nil
}

func resolveExisting(workspacePath, rel string) (string, *pathErr) {
	if perr := validateRelPath(rel); perr != nil {
		return "", perr
	}
	candidate := filepath.Join(workspacePath, rel)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newPathErr(ErrCategoryNotFound, "path does not exist: %s", rel)
		}
		return "", newPathErr(ErrCategoryUnknown, "resolve path %q: %v", rel, err)
	}
	if !isDescendant(workspacePath, resolved, false) {
		return "", newPathErr(ErrCategoryPathEscape, "path %q resolves outside workspace", rel)
	}
	return resolved, nil
}

func resolveWritable(workspacePath, rel string) (string, *pathErr) {
	if perr := validateRelPath(rel); perr != nil {
		return "", perr
	}
	candidate := filepath.Clean(filepath.Join(workspacePath, rel))
	if !syntacticDescendant(workspacePath, candidate) {
		return "", newPathErr(ErrCategoryPathEscape, "path %q resolves outside workspace", rel)
	}
	parent := filepath.Dir(candidate)
	ancestor, err := firstExistingAncestor(parent)
	if err != nil {
		return "", newPathErr(ErrCategoryIO, "locate ancestor of %q: %v", rel, err)
	}
	ancestorResolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", newPathErr(ErrCategoryIO, "resolve ancestor of %q: %v", rel, err)
	}
	if !isDescendant(workspacePath, ancestorResolved, true) {
		return "", newPathErr(ErrCategoryPathEscape, "ancestor of %q resolves outside workspace", rel)
	}
	if leafResolved, err := filepath.EvalSymlinks(candidate); err == nil {
		if !isDescendant(workspacePath, leafResolved, true) {
			return "", newPathErr(ErrCategoryPathEscape, "existing leaf at %q resolves outside workspace", rel)
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

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".apply_patch.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp to target: %w", err)
	}
	return nil
}

func containsNUL(p []byte) bool {
	for _, b := range p {
		if b == 0 {
			return true
		}
	}
	return false
}

func rejectDuplicateTopLevelKeys(raw []byte) error {
	return jsoncompat.RejectDuplicateTopLevelKeys(raw)
}

func failedResult(category, message string, files []FileResult) Result {
	if files == nil {
		files = []FileResult{}
	}
	return Result{
		Outcome: result.OutcomeFailed,
		Files:   files,
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
