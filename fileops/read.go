package fileops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/result"
)

// ReadArgs is the parsed input for file_read.
type ReadArgs struct {
	// Path is the workspace-relative file to read. Required.
	Path string `json:"path"`
}

// ReadResult is the model-facing envelope for file_read.
type ReadResult struct {
	BaseResult

	// Path echoes the requested path (workspace-relative). Populated
	// even on failure when the tool got far enough to know it.
	Path string `json:"path,omitempty"`

	// Content is the file's bytes as a UTF-8 string, truncated at
	// MaxOutputBytes. Empty on failure.
	Content string `json:"content,omitempty"`

	// ContentBytes is len(Content) for the model's convenience —
	// avoids a UTF-8 length count at the prompt-template layer.
	ContentBytes int `json:"content_bytes,omitempty"`

	// Truncated is true iff the file was larger than MaxOutputBytes
	// and Content holds only the leading prefix.
	Truncated bool `json:"truncated,omitempty"`
}

// UnmarshalJSON decodes ReadResult and preserves the original object in RawJSON.
func (r *ReadResult) UnmarshalJSON(raw []byte) error {
	type readResult ReadResult
	var decoded readResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*r = ReadResult(decoded)
	r.RawJSON = append(r.RawJSON[:0], raw...)
	return nil
}

// ReadTool implements file_read.
type ReadTool struct {
	workspacePath string
}

// NewReadTool constructs a ReadTool. Returns an error for an empty
// or relative workspace path (caller wiring bug).
func NewReadTool(workspacePath string) (*ReadTool, error) {
	if err := validateWorkspacePath(workspacePath); err != nil {
		return nil, err
	}
	return &ReadTool{workspacePath: canonicalizeWorkspace(workspacePath)}, nil
}

const readSchemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "path": {
      "type": "string",
      "minLength": 1,
      "description": "Workspace-relative path of the file to read."
    }
  },
  "required": ["path"]
}`

// ReadSchema returns a fresh copy of the file_read JSON Schema.
func ReadSchema() json.RawMessage {
	return append(json.RawMessage(nil), []byte(readSchemaJSON)...)
}

// Run reads the file at args.Path and returns a ReadResult.
func (t *ReadTool) Run(ctx context.Context, args ReadArgs) ReadResult {
	if t == nil {
		return ReadResult{BaseResult: failed(ErrCategoryValidation,
			"file_read tool is not configured for this run")}
	}
	if err := ctx.Err(); err != nil {
		return ReadResult{BaseResult: failed(contextErrCategory(err), err.Error())}
	}

	resolved, perr := resolveExisting(t.workspacePath, args.Path, false)
	if perr != nil {
		return ReadResult{
			BaseResult: failedFromPathErr(perr),
			Path:       args.Path,
		}
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return ReadResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("stat %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}
	if info.IsDir() {
		return ReadResult{
			BaseResult: failed(ErrCategoryIsDirectory,
				fmt.Sprintf("path %q is a directory; file_read expects a regular file", args.Path)),
			Path: args.Path,
		}
	}

	f, err := os.Open(resolved) //nolint:gosec // path verified by resolveExisting
	if err != nil {
		return ReadResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("open %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}
	defer func() { _ = f.Close() }()

	// Read up to MaxOutputBytes + 1 so we can detect truncation
	// with a single read budget. The +1 byte is discarded.
	limited := io.LimitReader(f, int64(MaxOutputBytes)+1)
	var sb strings.Builder
	n, err := io.Copy(&sb, limited)
	if err != nil {
		// io.Copy from a real file with a non-nil ctx can race
		// with shutdown; fold ctx errors into canceled.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ReadResult{
				BaseResult: failed(ErrCategoryCanceled, err.Error()),
				Path:       args.Path,
			}
		}
		return ReadResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("read %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}

	content := sb.String()
	truncated := false
	if n > int64(MaxOutputBytes) {
		// Trim to exactly MaxOutputBytes (the +1 byte we read past
		// the cap is in `content` if we hit the limit).
		content = content[:MaxOutputBytes]
		truncated = true
	}

	return ReadResult{
		BaseResult:   BaseResult{Outcome: result.OutcomeSucceeded},
		Path:         args.Path,
		Content:      content,
		ContentBytes: len(content),
		Truncated:    truncated,
	}
}

// Info returns the eino [*schema.ToolInfo] describing file_read's
// name, human-facing description, and JSON Schema for arguments.
// Called by eino at graph compile time; the ReAct loop uses Desc as
// the function description the model sees.
func (t *ReadTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return buildToolInfo(NameRead,
		"Read a workspace-relative file. Returns its UTF-8 content (truncated at 256 KiB) or a structured error envelope (path_escape, not_found, is_directory, io).",
		[]byte(readSchemaJSON))
}

// InvokableRun is the eino-friendly entry point. The variadic
// opts is required by the [tool.InvokableTool] ABI and is currently
// ignored — file_read has no per-call tool options.
func (t *ReadTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("file_read: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("file_read: parse arguments: %w", err)
	}
	var args ReadArgs
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("file_read: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("file_read: marshal result: %w", err)
	}
	return string(out), nil
}
