package fileops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/result"
)

// WriteArgs is the parsed input for file_write.
type WriteArgs struct {
	// Path is the workspace-relative target file. Required.
	Path string `json:"path"`

	// Content is the bytes to write. Required (use empty string to
	// truncate to zero bytes). Capped at MaxOutputBytes; oversize
	// content is rejected with ErrCategoryTooLarge.
	Content string `json:"content"`

	// CreateDirs, when true, runs mkdir -p on the parent directory
	// chain. Default false; an attempt to write into a non-existent
	// parent directory returns ErrCategoryNotFound so the model can
	// observe the structural problem before bulk-creating tree.
	CreateDirs bool `json:"create_dirs,omitempty"`
}

// WriteResult is the model-facing envelope for file_write.
type WriteResult struct {
	BaseResult

	// Path echoes the requested path (workspace-relative).
	Path string `json:"path,omitempty"`

	// BytesWritten is the size of the file post-write. Equal to
	// len(args.Content) on the success path; 0 on failure.
	BytesWritten int `json:"bytes_written,omitempty"`

	// Created reports whether the file did not exist before this
	// call. Useful for the model to distinguish "fresh file" from
	// "overwritten existing file" without a separate stat.
	Created bool `json:"created,omitempty"`
}

// UnmarshalJSON decodes WriteResult and preserves the original object in RawJSON.
func (r *WriteResult) UnmarshalJSON(raw []byte) error {
	type writeResult WriteResult
	var decoded writeResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*r = WriteResult(decoded)
	r.RawJSON = append(r.RawJSON[:0], raw...)
	return nil
}

// WriteTool implements file_write.
type WriteTool struct {
	workspacePath string
}

func NewWriteTool(workspacePath string) (*WriteTool, error) {
	if err := validateWorkspacePath(workspacePath); err != nil {
		return nil, err
	}
	return &WriteTool{workspacePath: canonicalizeWorkspace(workspacePath)}, nil
}

// File mode for newly-written files. 0o644 — owner read+write, group
// + world read. Workspace dir itself is 0o700 per workspace.Manager,
// so world bits on individual files do not widen access. The choice
// matches the convention many editors use; switching to 0o600 would
// surprise operators viewing workspace contents post-run.
const writeFileMode os.FileMode = 0o644

const writeSchemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "path": {
      "type": "string",
      "minLength": 1,
      "description": "Workspace-relative target file."
    },
    "content": {
      "type": "string",
      "description": "File content. May be empty (truncates the target to 0 bytes)."
    },
    "create_dirs": {
      "type": "boolean",
      "description": "If true, mkdir -p the parent chain. Default false."
    }
  },
  "required": ["path", "content"]
}`

// WriteSchema returns a fresh copy of the file_write JSON Schema.
func WriteSchema() json.RawMessage {
	return append(json.RawMessage(nil), []byte(writeSchemaJSON)...)
}

// Run writes args.Content to args.Path, truncating any existing
// file. Returns a WriteResult.
func (t *WriteTool) Run(ctx context.Context, args WriteArgs) WriteResult {
	if t == nil {
		return WriteResult{BaseResult: failed(ErrCategoryValidation,
			"file_write tool is not configured for this run")}
	}
	if err := ctx.Err(); err != nil {
		return WriteResult{BaseResult: failed(contextErrCategory(err), err.Error())}
	}
	if len(args.Content) > MaxOutputBytes {
		return WriteResult{
			BaseResult: failed(ErrCategoryTooLarge,
				fmt.Sprintf("content is %d bytes; max %d", len(args.Content), MaxOutputBytes)),
			Path: args.Path,
		}
	}

	resolved, perr := resolveWritable(t.workspacePath, args.Path, args.CreateDirs)
	if perr != nil {
		return WriteResult{
			BaseResult: failedFromPathErr(perr),
			Path:       args.Path,
		}
	}

	// Detect "did this exist before?" to populate Created. A stat
	// race is fine here — we don't depend on the answer for safety.
	created := false
	if _, err := os.Lstat(resolved); err != nil && errors.Is(err, os.ErrNotExist) {
		created = true
	}

	// Write atomically via a temp file + rename when the target
	// already exists. For brand-new files we use a direct create.
	// The atomic dance prevents an interrupted write from leaving
	// a half-truncated file (an empty file is much more confusing
	// to the model than no change at all).
	if err := atomicWrite(resolved, []byte(args.Content), writeFileMode); err != nil {
		return WriteResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("write %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}

	return WriteResult{
		BaseResult:   BaseResult{Outcome: result.OutcomeSucceeded},
		Path:         args.Path,
		BytesWritten: len(args.Content),
		Created:      created,
	}
}

// Info returns the eino [*schema.ToolInfo] for file_write.
func (t *WriteTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return buildToolInfo(NameWrite,
		"Write (create or overwrite) a workspace-relative file. Content is capped at 256 KiB. Optional create_dirs=true mkdir -p's the parent chain. Returns a structured error envelope on path_escape, not_found (missing parent), too_large, or io.",
		[]byte(writeSchemaJSON))
}

// InvokableRun is the eino-friendly entry point. opts is ignored —
// file_write has no per-call tool options.
func (t *WriteTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("file_write: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("file_write: parse arguments: %w", err)
	}
	var args WriteArgs
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("file_write: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("file_write: marshal result: %w", err)
	}
	return string(out), nil
}

// atomicWrite writes data to path via a sibling temp file + rename.
// On rename failure (cross-device, permission, etc.) the temp file
// is cleaned up so we don't leak inode noise into the workspace.
//
// The temp file lives in the same directory as the target so the
// rename is intra-filesystem (atomic on POSIX). Mode is applied
// before the rename so the final file has the requested mode from
// the moment it appears at the target name.
//
// Crash semantics: "atomic" here means in-process atomicity — a
// concurrent reader sees either the pre-rename target or the new
// content, never a partial write. We deliberately do NOT call
// tmp.Sync() before close: the workspace is ephemeral by design
// (per-issue scratch space discarded after the run completes), so
// surviving a power-loss with a zero-length file is not a
// correctness regression. If a future deployment makes the
// workspace persistent (e.g., cached across runs), revisit this.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".file_write.*.tmp")
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
		_ = os.Remove(tmpPath) //nolint:gosec // tmpPath created by os.CreateTemp inside dir
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil { //nolint:gosec // both paths verified by resolveWritable + CreateTemp
		_ = os.Remove(tmpPath) //nolint:gosec // tmpPath created by os.CreateTemp inside dir
		return fmt.Errorf("rename temp to target: %w", err)
	}
	return nil
}
