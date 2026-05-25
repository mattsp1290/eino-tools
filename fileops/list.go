package fileops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/result"
)

// ListArgs is the parsed input for file_list.
type ListArgs struct {
	// Path is the workspace-relative directory to list. Optional —
	// empty string and "." both list the workspace root.
	Path string `json:"path,omitempty"`

	// Recursive, when true, walks the directory tree and returns
	// every descendant path. Default false (immediate children only).
	Recursive bool `json:"recursive,omitempty"`
}

// ListEntry describes one filesystem entry. Path is workspace-
// relative for cross-call portability — the model can hand the
// same string to file_read / file_edit without translation.
type ListEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir,omitempty"`
}

// ListResult is the model-facing envelope for file_list.
type ListResult struct {
	BaseResult

	// Path echoes the requested directory (workspace-relative).
	// "." for the workspace root.
	Path string `json:"path,omitempty"`

	// Entries are sorted lexicographically by Path. Capped at
	// MaxListEntries; truncation surfaced via Truncated below.
	Entries []ListEntry `json:"entries"`

	// Truncated is true iff the directory contains more than
	// MaxListEntries entries (or, for recursive=true, more than
	// MaxListEntries descendants). Entries holds the leading
	// MaxListEntries entries in sorted order.
	Truncated bool `json:"truncated,omitempty"`
}

// UnmarshalJSON decodes ListResult and preserves the original object in RawJSON.
func (r *ListResult) UnmarshalJSON(raw []byte) error {
	type listResult ListResult
	var decoded listResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*r = ListResult(decoded)
	r.RawJSON = append(r.RawJSON[:0], raw...)
	return nil
}

// ListTool implements file_list.
type ListTool struct {
	workspacePath string
}

func NewListTool(workspacePath string) (*ListTool, error) {
	if err := validateWorkspacePath(workspacePath); err != nil {
		return nil, err
	}
	return &ListTool{workspacePath: canonicalizeWorkspace(workspacePath)}, nil
}

const listSchemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "path": {
      "type": "string",
      "minLength": 1,
      "description": "Workspace-relative directory. Use '.' for the workspace root. Omit the field entirely to default to '.'."
    },
    "recursive": {
      "type": "boolean",
      "description": "If true, walk descendants. Default false (immediate children only)."
    }
  }
}`

func ListSchema() json.RawMessage {
	return append(json.RawMessage(nil), []byte(listSchemaJSON)...)
}

// Run lists args.Path. Returns sorted entries (workspace-relative).
func (t *ListTool) Run(ctx context.Context, args ListArgs) ListResult {
	if t == nil {
		return ListResult{
			BaseResult: failed(ErrCategoryValidation,
				"file_list tool is not configured for this run"),
			Entries: []ListEntry{},
		}
	}
	if err := ctx.Err(); err != nil {
		return ListResult{
			BaseResult: failed(contextErrCategory(err), err.Error()),
			Entries:    []ListEntry{},
		}
	}

	rel := args.Path
	if rel == "" {
		rel = "."
	}

	// Reject absolute paths up front — would be surprising for
	// the model to discover at the resolveExisting boundary.
	if rel != "." {
		if perr := validateRelPath(rel); perr != nil {
			return ListResult{
				BaseResult: failedFromPathErr(perr),
				Path:       rel,
				Entries:    []ListEntry{},
			}
		}
	}

	var resolved string
	if rel == "." {
		resolved = t.workspacePath
	} else {
		var perr *pathErr
		resolved, perr = resolveExisting(t.workspacePath, rel, false)
		if perr != nil {
			return ListResult{
				BaseResult: failedFromPathErr(perr),
				Path:       rel,
				Entries:    []ListEntry{},
			}
		}
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return ListResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("stat %q: %v", rel, err)),
			Path:    rel,
			Entries: []ListEntry{},
		}
	}
	if !info.IsDir() {
		return ListResult{
			BaseResult: failed(ErrCategoryNotDirectory,
				fmt.Sprintf("path %q is not a directory; file_list expects a directory", rel)),
			Path:    rel,
			Entries: []ListEntry{},
		}
	}

	var entries []ListEntry
	truncated := false

	if args.Recursive {
		// filepath.WalkDir is the cheaper-than-Walk variant that
		// avoids per-entry os.Lstat. We emit workspace-relative
		// paths with forward slashes (filepath uses native
		// separator; on darwin/linux they match).
		walkErr := filepath.WalkDir(resolved, func(p string, d os.DirEntry, werr error) error {
			if werr != nil {
				return werr
			}
			if p == resolved {
				return nil // skip the root itself
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(entries) >= MaxListEntries {
				truncated = true
				return filepath.SkipAll
			}
			rrel, rerr := filepath.Rel(t.workspacePath, p)
			if rerr != nil {
				return rerr
			}
			entries = append(entries, ListEntry{
				Path:  rrel,
				IsDir: d.IsDir(),
			})
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, filepath.SkipAll) {
			if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
				return ListResult{
					BaseResult: failed(ErrCategoryCanceled, walkErr.Error()),
					Path:       rel,
					Entries:    []ListEntry{},
				}
			}
			return ListResult{
				BaseResult: failed(ErrCategoryIO,
					fmt.Sprintf("walk %q: %v", rel, walkErr)),
				Path:    rel,
				Entries: []ListEntry{},
			}
		}
	} else {
		dirEntries, err := os.ReadDir(resolved)
		if err != nil {
			return ListResult{
				BaseResult: failed(ErrCategoryIO,
					fmt.Sprintf("read directory %q: %v", rel, err)),
				Path:    rel,
				Entries: []ListEntry{},
			}
		}
		// os.ReadDir returns sorted by name already (since Go 1.17),
		// but we don't rely on that — sort post-pruning so the
		// truncation case and the recursive case have identical
		// ordering semantics.
		for _, d := range dirEntries {
			if len(entries) >= MaxListEntries {
				truncated = true
				break
			}
			joined := filepath.Join(rel, d.Name())
			if rel == "." {
				joined = d.Name()
			}
			entries = append(entries, ListEntry{
				Path:  joined,
				IsDir: d.IsDir(),
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	if entries == nil {
		entries = []ListEntry{}
	}

	return ListResult{
		BaseResult: BaseResult{Outcome: result.OutcomeSucceeded},
		Path:       rel,
		Entries:    entries,
		Truncated:  truncated,
	}
}

// Info returns the eino [*schema.ToolInfo] for file_list.
func (t *ListTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return buildToolInfo(NameList,
		"List directory entries under a workspace-relative path (empty or \".\" lists the workspace root). Output is sorted and capped at 5000 entries; oversize results set truncated=true.",
		[]byte(listSchemaJSON))
}

// InvokableRun is the eino-friendly entry point. opts is ignored —
// file_list has no per-call tool options.
func (t *ListTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("file_list: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("file_list: parse arguments: %w", err)
	}
	var args ListArgs
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("file_list: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("file_list: marshal result: %w", err)
	}
	return string(out), nil
}
