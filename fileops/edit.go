package fileops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/result"
)

// EditArgs is the parsed input for file_edit.
type EditArgs struct {
	// Path is the workspace-relative file to edit. Required, must
	// already exist.
	Path string `json:"path"`

	// Anchor is the literal substring to find. Must match exactly
	// once in the file. No regex, no glob — the model is expected
	// to extend Anchor with surrounding context to disambiguate
	// when needed.
	Anchor string `json:"anchor"`

	// Replacement is what to substitute for Anchor. May be empty
	// (deletes the anchor), may be longer than Anchor.
	Replacement string `json:"replacement"`
}

// EditResult is the model-facing envelope for file_edit.
type EditResult struct {
	BaseResult

	// Path echoes the requested path.
	Path string `json:"path,omitempty"`

	// BytesWritten is the size of the file post-edit.
	BytesWritten int `json:"bytes_written,omitempty"`

	// AnchorOccurrences is the count of anchor matches found in
	// the pre-edit file content. Always 1 on the success path;
	// populated on the anchor_not_found (0) and anchor_ambiguous
	// (≥2) failure paths so the model can confirm what the tool
	// saw.
	//
	// Note: NOT `omitempty` — the 0 value is meaningful on the
	// anchor_not_found path and must serialize. Drops on success
	// would be acceptable, but consistency across error paths is
	// the higher-value contract.
	AnchorOccurrences int `json:"anchor_occurrences"`
}

// UnmarshalJSON decodes EditResult and preserves the original object in RawJSON.
func (r *EditResult) UnmarshalJSON(raw []byte) error {
	type editResult EditResult
	var decoded editResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*r = EditResult(decoded)
	r.RawJSON = append(r.RawJSON[:0], raw...)
	return nil
}

// EditTool implements file_edit.
type EditTool struct {
	workspacePath string
}

func NewEditTool(workspacePath string) (*EditTool, error) {
	if err := validateWorkspacePath(workspacePath); err != nil {
		return nil, err
	}
	return &EditTool{workspacePath: canonicalizeWorkspace(workspacePath)}, nil
}

const editSchemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "path": {
      "type": "string",
      "minLength": 1,
      "description": "Workspace-relative file to edit. Must exist."
    },
    "anchor": {
      "type": "string",
      "minLength": 1,
      "description": "Literal substring to find. Must match exactly once. No regex."
    },
    "replacement": {
      "type": "string",
      "description": "Substitute for the anchor. May be empty."
    }
  },
  "required": ["path", "anchor", "replacement"]
}`

func EditSchema() json.RawMessage {
	return append(json.RawMessage(nil), []byte(editSchemaJSON)...)
}

// Run reads args.Path, replaces the unique occurrence of args.Anchor
// with args.Replacement, and writes back atomically.
func (t *EditTool) Run(ctx context.Context, args EditArgs) EditResult {
	if t == nil {
		return EditResult{BaseResult: failed(ErrCategoryValidation,
			"file_edit tool is not configured for this run")}
	}
	if err := ctx.Err(); err != nil {
		return EditResult{BaseResult: failed(contextErrCategory(err), err.Error())}
	}
	if args.Anchor == "" {
		return EditResult{
			BaseResult: failed(ErrCategoryValidation, "anchor is required and must be non-empty"),
			Path:       args.Path,
		}
	}

	resolved, perr := resolveExisting(t.workspacePath, args.Path, false)
	if perr != nil {
		return EditResult{
			BaseResult: failedFromPathErr(perr),
			Path:       args.Path,
		}
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return EditResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("stat %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}
	if info.IsDir() {
		return EditResult{
			BaseResult: failed(ErrCategoryIsDirectory,
				fmt.Sprintf("path %q is a directory; file_edit expects a regular file", args.Path)),
			Path: args.Path,
		}
	}

	// Read the entire file. We deliberately do NOT cap to
	// MaxOutputBytes here — capping would silently mis-edit the
	// tail of a long file. Instead we reject the operation with
	// ErrCategoryTooLarge for files above the cap; the model
	// must use file_write to replace large files wholesale.
	if info.Size() > int64(MaxOutputBytes) {
		return EditResult{
			BaseResult: failed(ErrCategoryTooLarge,
				fmt.Sprintf("file %q is %d bytes; file_edit max is %d (use file_write for larger replacements)",
					args.Path, info.Size(), MaxOutputBytes)),
			Path: args.Path,
		}
	}

	contentBytes, err := os.ReadFile(resolved) //nolint:gosec // path verified by resolveExisting
	if err != nil {
		return EditResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("read %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}

	// Operate on []byte throughout — avoids the ~4× memory peak of
	// the previous string-round-trip pipeline ([]byte → string for
	// strings.Count + Replace → []byte for atomicWrite). At
	// MaxOutputBytes the savings are real (256 KB × 4 = 1 MB transient
	// per call); at any future cap doubling the savings compound.
	anchor := []byte(args.Anchor)
	occurrences := bytes.Count(contentBytes, anchor)
	switch occurrences {
	case 0:
		return EditResult{
			BaseResult: failed(ErrCategoryAnchorNotFound,
				fmt.Sprintf("anchor not found in %q", args.Path)),
			Path:              args.Path,
			AnchorOccurrences: 0,
		}
	case 1:
		// happy path; fall through
	default:
		return EditResult{
			BaseResult: failed(ErrCategoryAnchorAmbiguous,
				fmt.Sprintf("anchor matched %d times in %q; extend the anchor to disambiguate",
					occurrences, args.Path)),
			Path:              args.Path,
			AnchorOccurrences: occurrences,
		}
	}

	updated := bytes.Replace(contentBytes, anchor, []byte(args.Replacement), 1)
	if len(updated) > MaxOutputBytes {
		return EditResult{
			BaseResult: failed(ErrCategoryTooLarge,
				fmt.Sprintf("post-edit file would be %d bytes; max %d",
					len(updated), MaxOutputBytes)),
			Path: args.Path,
		}
	}

	// Preserve the existing file mode so the edit is mode-preserving
	// (chmod 0o755 on a script must survive a no-op edit).
	if err := atomicWrite(resolved, updated, info.Mode().Perm()); err != nil {
		return EditResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("write %q: %v", args.Path, err)),
			Path: args.Path,
		}
	}

	return EditResult{
		BaseResult:        BaseResult{Outcome: result.OutcomeSucceeded},
		Path:              args.Path,
		BytesWritten:      len(updated),
		AnchorOccurrences: 1,
	}
}

// Info returns the eino [*schema.ToolInfo] for file_edit.
func (t *EditTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return buildToolInfo(NameEdit,
		"Edit a workspace-relative file in place by anchored substring replacement. The anchor MUST appear exactly once in the file. Returns structured error envelopes on path_escape, not_found, is_directory, anchor_not_found, anchor_ambiguous, too_large, or io.",
		[]byte(editSchemaJSON))
}

// InvokableRun is the eino-friendly entry point. opts is ignored —
// file_edit has no per-call tool options.
func (t *EditTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("file_edit: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("file_edit: parse arguments: %w", err)
	}
	var args EditArgs
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("file_edit: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("file_edit: marshal result: %w", err)
	}
	return string(out), nil
}
