package fileops

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/result"
)

// ReadArgs is the parsed input for file_read.
type ReadArgs struct {
	// Path is the workspace-relative file to read. Required.
	Path string `json:"path"`

	// Offset is the optional 1-based starting line for a line-windowed read.
	// When omitted with Limit, file_read preserves the legacy prefix result.
	Offset *int `json:"offset,omitempty"`

	// Limit is the optional number of lines to return for a line-windowed read.
	// When omitted with Offset present, DefaultReadWindowLines is used.
	Limit *int `json:"limit,omitempty"`
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
	// and Content holds only the leading prefix, or iff a line-windowed
	// response hit its line, byte, or long-line caps.
	Truncated bool `json:"truncated,omitempty"`

	// NumberedContent is populated for line-windowed reads. It prefixes each
	// selected line with "<line_number>: " while Content remains raw text.
	NumberedContent string `json:"numbered_content,omitempty"`

	// LineStart and LineEnd bound the selected line window. They are populated
	// only for line-windowed successful reads.
	LineStart int `json:"line_start,omitempty"`
	LineEnd   int `json:"line_end,omitempty"`

	// TotalLines is the total number of logical text lines in the file.
	TotalLines int `json:"total_lines,omitempty"`

	// NextOffset is populated when another line-windowed read can continue
	// after the returned selection.
	NextOffset int `json:"next_offset,omitempty"`

	// LineTruncated reports whether at least one retained line hit
	// MaxReadLineBytes and carries an inline truncation marker.
	LineTruncated bool `json:"line_truncated,omitempty"`

	// TruncationReason names the cap that truncated a line-windowed result:
	// "lines", "bytes", "line", or "prefix".
	TruncationReason string `json:"truncation_reason,omitempty"`
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
    },
    "offset": {
      "type": "integer",
      "minimum": 1,
      "description": "Optional 1-based starting line for a line-windowed read. When omitted, legacy prefix reads are preserved unless limit is present."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 5000,
      "description": "Optional number of lines for a line-windowed read. Default 2000 when offset or limit is present; cap is 5000."
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
	if args.Offset != nil || args.Limit != nil {
		return t.runLineWindow(ctx, args, resolved)
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

	contentBytes := make([]byte, 0, MaxOutputBytes+1)
	limited := io.LimitReader(f, int64(MaxOutputBytes)+1)
	n, err := io.Copy((*byteSliceWriter)(&contentBytes), limited)
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

	truncated := n > int64(MaxOutputBytes)
	checkBytes := contentBytes
	if truncated {
		checkBytes = []byte(trimUTF8(string(contentBytes[:MaxOutputBytes])))
	}
	if bytesLookBinary(checkBytes) || containsNUL(contentBytes) {
		return ReadResult{
			BaseResult: failed(ErrCategoryBinary,
				fmt.Sprintf("file %q appears to be binary or non-UTF-8 text", args.Path)),
			Path: args.Path,
		}
	}

	content := string(checkBytes)

	return ReadResult{
		BaseResult:       BaseResult{Outcome: result.OutcomeSucceeded},
		Path:             args.Path,
		Content:          content,
		ContentBytes:     len(content),
		Truncated:        truncated,
		TruncationReason: prefixTruncationReason(truncated),
	}
}

func (t *ReadTool) runLineWindow(ctx context.Context, args ReadArgs, resolved string) ReadResult {
	offset := 1
	if args.Offset != nil {
		offset = *args.Offset
	}
	if offset < 1 {
		return ReadResult{
			BaseResult: failed(ErrCategoryValidation,
				fmt.Sprintf("offset must be >= 1, got %d", offset)),
			Path: args.Path,
		}
	}
	limit := DefaultReadWindowLines
	if args.Limit != nil {
		limit = *args.Limit
	}
	if limit < 1 {
		return ReadResult{
			BaseResult: failed(ErrCategoryValidation,
				fmt.Sprintf("limit must be >= 1, got %d", limit)),
			Path: args.Path,
		}
	}
	if limit > MaxReadWindowLines {
		return ReadResult{
			BaseResult: failed(ErrCategoryValidation,
				fmt.Sprintf("limit %d exceeds maximum %d", limit, MaxReadWindowLines)),
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

	var content strings.Builder
	var numbered strings.Builder
	reader := bufio.NewReader(f)
	totalLines := 0
	lineEnd := 0
	contentBytes := 0
	truncated := false
	lineTruncated := false
	truncReason := ""
	byteCapHit := false

	for {
		if err := ctx.Err(); err != nil {
			return ReadResult{
				BaseResult: failed(contextErrCategory(err), err.Error()),
				Path:       args.Path,
			}
		}

		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			totalLines++
			if strings.ContainsRune(line, 0) || !utf8.ValidString(line) {
				return ReadResult{
					BaseResult: failed(ErrCategoryBinary,
						fmt.Sprintf("file %q appears to be binary or non-UTF-8 text", args.Path)),
					Path: args.Path,
				}
			}
			if totalLines >= offset && totalLines < offset+limit && !byteCapHit {
				retained, wasLineTruncated := truncateReadLine(line)
				if contentBytes+len(retained) > MaxOutputBytes {
					byteCapHit = true
					truncated = true
					truncReason = "bytes"
				} else {
					if wasLineTruncated {
						lineTruncated = true
						if truncReason == "" {
							truncReason = "line"
						}
					}
					content.WriteString(retained)
					numbered.WriteString(fmt.Sprintf("%d: %s", totalLines, retained))
					contentBytes += len(retained)
					lineEnd = totalLines
				}
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		return ReadResult{
			BaseResult: failed(ErrCategoryIO,
				fmt.Sprintf("read %q: %v", args.Path, readErr)),
			Path: args.Path,
		}
	}

	if offset > totalLines {
		return ReadResult{
			BaseResult: failed(ErrCategoryValidation,
				fmt.Sprintf("offset %d is past EOF; file has %d total lines", offset, totalLines)),
			Path: args.Path,
		}
	}
	if lineEnd == 0 {
		lineEnd = offset - 1
	}

	nextOffset := 0
	if lineEnd < totalLines {
		truncated = true
		if truncReason == "" || truncReason == "line" {
			truncReason = "lines"
		}
		nextOffset = lineEnd + 1
	}
	if lineTruncated {
		truncated = true
	}

	return ReadResult{
		BaseResult:       BaseResult{Outcome: result.OutcomeSucceeded},
		Path:             args.Path,
		Content:          content.String(),
		ContentBytes:     content.Len(),
		Truncated:        truncated,
		NumberedContent:  numbered.String(),
		LineStart:        offset,
		LineEnd:          lineEnd,
		TotalLines:       totalLines,
		NextOffset:       nextOffset,
		LineTruncated:    lineTruncated,
		TruncationReason: truncReason,
	}
}

// Info returns the eino [*schema.ToolInfo] describing file_read's
// name, human-facing description, and JSON Schema for arguments.
// Called by eino at graph compile time; the ReAct loop uses Desc as
// the function description the model sees.
func (t *ReadTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return buildToolInfo(NameRead,
		"Read a workspace-relative UTF-8 text file. Plain {path} calls return the leading content prefix capped at 256 KiB. Supplying offset and/or limit returns a line-window with raw and numbered content. Returns structured errors including path_escape, not_found, is_directory, binary, validation, and io.",
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

type byteSliceWriter []byte

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	*w = append(*w, p...)
	return len(p), nil
}

func bytesLookBinary(p []byte) bool {
	return !utf8.Valid(p)
}

func containsNUL(p []byte) bool {
	for _, b := range p {
		if b == 0 {
			return true
		}
	}
	return false
}

func truncateReadLine(line string) (string, bool) {
	if len(line) <= MaxReadLineBytes {
		return line, false
	}
	hasNewline := strings.HasSuffix(line, "\n")
	body := line
	lineEnding := ""
	if hasNewline {
		lineEnding = "\n"
		body = strings.TrimSuffix(body, "\n")
		if strings.HasSuffix(body, "\r") {
			body = strings.TrimSuffix(body, "\r")
			lineEnding = "\r\n"
		}
	}
	body = trimUTF8(body[:MaxReadLineBytes])
	return body + "...(line truncated)" + lineEnding, true
}

func trimUTF8(s string) string {
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func prefixTruncationReason(truncated bool) string {
	if truncated {
		return "prefix"
	}
	return ""
}
