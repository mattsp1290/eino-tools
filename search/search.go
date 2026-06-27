package search

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
	"github.com/mattsp1290/eino-tools/result"
)

// Name is the model-facing tool name.
const Name = "search"

const (
	DefaultTimeoutSeconds = 60
	MaxTimeoutSeconds     = 600
	MaxMatches            = 200
	MaxSearchLimit        = 1000
	MaxContextLines       = 20
	MaxLineBytes          = 4 * 1024
	MaxResultBytes        = 256 * 1024
	MaxStderrBytes        = 4 * 1024

	scannerLineCap       = 8 * 1024 * 1024
	waitDelayAfterCancel = 5 * time.Second
	defaultRgBinary      = "rg"
)

// Args is the parsed input shape for the tool.
type Args struct {
	Pattern        string `json:"pattern"`
	Path           string `json:"path,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Glob           Globs  `json:"glob,omitempty"`
	Literal        bool   `json:"literal,omitempty"`
	IgnoreCase     bool   `json:"ignore_case,omitempty"`
	Context        int    `json:"context,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

// Globs accepts either a single JSON string or an array of strings.
type Globs []string

func (g *Globs) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		*g = Globs{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return err
	}
	*g = Globs(many)
	return nil
}

// Result is the structured envelope returned to the model.
type Result struct {
	Outcome          result.Outcome  `json:"outcome"`
	Matches          []Match         `json:"matches"`
	MatchCount       int             `json:"match_count"`
	Truncated        bool            `json:"truncated,omitempty"`
	TruncationReason string          `json:"truncation_reason,omitempty"`
	DurationMS       int64           `json:"duration_ms"`
	TimedOut         bool            `json:"timed_out,omitempty"`
	Partial          bool            `json:"partial,omitempty"`
	Error            *ResultError    `json:"error,omitempty"`
	RawJSON          json.RawMessage `json:"-"`
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

// Match is one ripgrep match event after line/submatch normalization.
type Match struct {
	Path          string        `json:"path"`
	LineNumber    int           `json:"line_number"`
	Line          string        `json:"line"`
	LineTruncated bool          `json:"line_truncated,omitempty"`
	Submatches    []Submatch    `json:"submatches,omitempty"`
	Before        []ContextLine `json:"before,omitempty"`
	After         []ContextLine `json:"after,omitempty"`
}

// ContextLine is one before/after context line around a match.
type ContextLine struct {
	LineNumber    int    `json:"line_number"`
	Line          string `json:"line"`
	LineTruncated bool   `json:"line_truncated,omitempty"`
}

// Submatch is a regex span within a matched line.
type Submatch struct {
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// ResultError is the structured failure envelope nested inside Result.
type ResultError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

const (
	ErrCategoryValidation     = "validation"
	ErrCategoryPathEscape     = "path_escape"
	ErrCategoryNotFound       = "not_found"
	ErrCategoryInvalidPattern = "invalid_pattern"
	ErrCategoryTimeout        = "timeout"
	ErrCategoryCanceled       = "canceled"
	ErrCategoryExecFailed     = "exec_failed"
	ErrCategoryUnknown        = "unknown"
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
	case ErrCategoryTimeout, ErrCategoryUnknown:
		return true
	default:
		return false
	}
}

// Tool searches a configured workspace with ripgrep.
type Tool struct {
	workspacePath string
	rgBinary      string
}

// New constructs a Tool from an absolute, resolvable workspace path.
func New(workspacePath string) (*Tool, error) {
	if workspacePath == "" {
		return nil, errors.New("search: workspace path is required")
	}
	if !filepath.IsAbs(workspacePath) {
		return nil, fmt.Errorf("search: workspace path must be absolute, got %q", workspacePath)
	}
	resolved, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("search: resolve workspace path %q: %w", workspacePath, err)
	}
	return &Tool{workspacePath: resolved, rgBinary: defaultRgBinary}, nil
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "pattern": {
      "type": "string",
      "minLength": 1,
      "description": "Regex pattern. Forwarded to ripgrep via -e. Inline flags such as (?i) are supported."
    },
    "path": {
      "type": "string",
      "description": "Workspace-relative search root. Optional; empty or \".\" searches the entire workspace."
    },
    "timeout_seconds": {
      "type": "integer",
      "minimum": 0,
      "maximum": 600,
      "description": "Per-call timeout in seconds. 0 or omitted applies the default (60s). Cap is 600s."
    },
    "glob": {
      "oneOf": [
        {"type": "string", "minLength": 1},
        {"type": "array", "items": {"type": "string", "minLength": 1}, "minItems": 1}
      ],
      "description": "Optional ripgrep include glob or globs, forwarded as repeated -g filters."
    },
    "literal": {
      "type": "boolean",
      "description": "If true, search for pattern as a fixed string via ripgrep -F. Default false."
    },
    "ignore_case": {
      "type": "boolean",
      "description": "If true, search case-insensitively via ripgrep -i. Default false."
    },
    "context": {
      "type": "integer",
      "minimum": 0,
      "maximum": 20,
      "description": "Number of before and after context lines to return. Default 0; cap is 20."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "description": "Maximum matches to return. Default 200; cap is 1000."
    }
  },
  "required": ["pattern"]
}`

// Schema returns a fresh JSON Schema copy for search arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

// Run executes ripgrep and returns parsed match results.
func (t *Tool) Run(ctx context.Context, args Args) Result {
	start := time.Now()
	if t == nil {
		return failedResult(ErrCategoryValidation, "search tool is not configured for this run", start)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return failedResult(ErrCategoryValidation, "pattern is required and must be non-empty", start)
	}
	if strings.ContainsRune(args.Pattern, 0) {
		return failedResult(ErrCategoryValidation, "pattern contains NUL byte", start)
	}
	if args.TimeoutSeconds < 0 {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("timeout_seconds must be non-negative, got %d", args.TimeoutSeconds), start)
	}
	if args.TimeoutSeconds > MaxTimeoutSeconds {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("timeout_seconds %d exceeds maximum %d", args.TimeoutSeconds, MaxTimeoutSeconds), start)
	}
	if args.Context < 0 {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("context must be non-negative, got %d", args.Context), start)
	}
	if args.Context > MaxContextLines {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("context %d exceeds maximum %d", args.Context, MaxContextLines), start)
	}
	limit := args.Limit
	if limit == 0 {
		limit = MaxMatches
	}
	if limit < 0 {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("limit must be non-negative, got %d", args.Limit), start)
	}
	if limit > MaxSearchLimit {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("limit %d exceeds maximum %d", args.Limit, MaxSearchLimit), start)
	}
	for _, glob := range args.Glob {
		if strings.TrimSpace(glob) == "" {
			return failedResult(ErrCategoryValidation, "glob entries must be non-empty", start)
		}
		if strings.ContainsRune(glob, 0) {
			return failedResult(ErrCategoryValidation, "glob contains NUL byte", start)
		}
	}
	if err := ctx.Err(); err != nil {
		return failedResult(ErrCategoryCanceled, err.Error(), start)
	}

	rel := args.Path
	if rel == "" {
		rel = "."
	}
	rel = filepath.Clean(rel)
	if _, perr := resolveExisting(t.workspacePath, rel); perr != nil {
		return failedResult(perr.category, perr.message, start)
	}

	timeout := time.Duration(args.TimeoutSeconds) * time.Second
	if args.TimeoutSeconds == 0 {
		timeout = time.Duration(DefaultTimeoutSeconds) * time.Second
	}

	parentCtx := ctx
	runCtx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	rgArgs := []string{"--json"}
	if args.Literal {
		rgArgs = append(rgArgs, "-F")
	}
	if args.IgnoreCase {
		rgArgs = append(rgArgs, "-i")
	}
	if args.Context > 0 {
		rgArgs = append(rgArgs, "-C", strconv.Itoa(args.Context))
	}
	for _, glob := range args.Glob {
		rgArgs = append(rgArgs, "-g", glob)
	}
	rgArgs = append(rgArgs, "-e", args.Pattern, "--", rel)

	// gosec G204: ripgrep execution is intentional. Pattern, globs, and path
	// are passed as separate argv entries with -e/-g/-- boundaries; workspace
	// containment is enforced by resolveExisting before exec.
	cmd := exec.CommandContext(runCtx, t.rgBinary, rgArgs...) //nolint:gosec
	cmd.Dir = t.workspacePath
	cmd.Stdin = bytes.NewReader(nil)
	cmd.WaitDelay = waitDelayAfterCancel

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return failedResult(ErrCategoryExecFailed, fmt.Sprintf("rg stdout pipe: %v", err), start)
	}
	stderrBuf := newCappedBuffer(MaxStderrBytes)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		if isExecNotFound(err) {
			return failedResult(ErrCategoryExecFailed,
				fmt.Sprintf("rg binary not found on PATH: %v", err), start)
		}
		return failedResult(ErrCategoryExecFailed, fmt.Sprintf("rg start: %v", err), start)
	}

	matches, truncated, truncReason, parseErr := parseStream(stdoutPipe, parseOptions{
		limit:             limit,
		includeSubmatches: !args.Literal,
		contextLines:      args.Context,
	})
	if truncated || parseErr != nil {
		cancel()
		_, _ = io.Copy(io.Discard, stdoutPipe)
	}

	waitErr := cmd.Wait()

	res := Result{
		Matches:    matches,
		MatchCount: len(matches),
		DurationMS: time.Since(start).Milliseconds(),
	}
	if truncated {
		res.Truncated = true
		res.TruncationReason = truncReason
	}

	if !truncated {
		if perr := parentCtx.Err(); perr != nil {
			res.Outcome = result.OutcomeFailed
			res.Error = &ResultError{Category: ErrCategoryCanceled, Message: perr.Error()}
			return res
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			res.Outcome = result.OutcomeFailed
			res.TimedOut = true
			res.Error = &ResultError{
				Category: ErrCategoryTimeout,
				Message:  fmt.Sprintf("rg exceeded %ds timeout", int(timeout.Seconds())),
			}
			return res
		}
		if parseErr != nil {
			res.Outcome = result.OutcomeFailed
			res.Error = &ResultError{
				Category: ErrCategoryUnknown,
				Message:  fmt.Sprintf("parse rg output: %v", parseErr),
			}
			return res
		}
	}

	exitCode := exitCodeOf(waitErr)
	switch {
	case waitErr == nil || exitCode == 0:
		res.Outcome = result.OutcomeSucceeded
		return res
	case exitCode == 1:
		res.Outcome = result.OutcomeSucceeded
		return res
	case truncated:
		res.Outcome = result.OutcomeSucceeded
		return res
	case exitCode == 2:
		if len(matches) > 0 {
			res.Outcome = result.OutcomeSucceeded
			res.Partial = true
			res.Error = &ResultError{Category: ErrCategoryExecFailed, Message: rgErrorMessage(stderrBuf, waitErr)}
			return res
		}
		res.Outcome = result.OutcomeFailed
		res.Error = &ResultError{Category: ErrCategoryInvalidPattern, Message: rgErrorMessage(stderrBuf, waitErr)}
		return res
	default:
		if len(matches) > 0 {
			res.Outcome = result.OutcomeSucceeded
			res.Partial = true
			res.Error = &ResultError{Category: ErrCategoryExecFailed, Message: rgErrorMessage(stderrBuf, waitErr)}
			return res
		}
		res.Outcome = result.OutcomeFailed
		res.Error = &ResultError{
			Category: ErrCategoryExecFailed,
			Message: fmt.Sprintf("rg exited with unexpected status %d: %s",
				exitCode, rgErrorMessage(stderrBuf, waitErr)),
		}
		return res
	}
}

// Info returns the Eino ToolInfo for search.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("search: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Search workspace files via ripgrep. Defaults to regex mode and preserves existing pattern/path/timeout behavior. Optional glob filters, literal mode, ignore-case mode, context lines, and match limit are supported. Per-call timeout defaults to 60s and is capped at 600s.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	args := Args{}
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("search: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("search: parse arguments: %w", err)
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("search: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("search: marshal result: %w", err)
	}
	return string(out), nil
}

type rgEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgMatchData struct {
	Path       rgText       `json:"path"`
	Lines      rgText       `json:"lines"`
	LineNumber int          `json:"line_number"`
	Submatches []rgSubmatch `json:"submatches"`
}

type rgSubmatch struct {
	Match rgText `json:"match"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type rgText struct {
	Text  *string `json:"text,omitempty"`
	Bytes *string `json:"bytes,omitempty"`
}

func (t rgText) decode() string {
	if t.Text != nil {
		if utf8.ValidString(*t.Text) {
			return *t.Text
		}
		return strings.ToValidUTF8(*t.Text, "\uFFFD")
	}
	if t.Bytes != nil {
		raw, err := base64.StdEncoding.DecodeString(*t.Bytes)
		if err != nil {
			return ""
		}
		return strings.ToValidUTF8(string(raw), "\uFFFD")
	}
	return ""
}

type parseOptions struct {
	limit             int
	includeSubmatches bool
	contextLines      int
}

func parseStream(r io.Reader, opts parseOptions) ([]Match, bool, string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), scannerLineCap)

	if opts.limit <= 0 {
		opts.limit = MaxMatches
	}
	matches := make([]Match, 0, min(opts.limit, MaxMatches))
	pendingContext := make([]ContextLine, 0)
	lastMatchIndex := -1
	bytesUsed := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev rgEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type == "begin" || ev.Type == "end" {
			if ev.Type == "end" && lastMatchIndex >= 0 {
				attachTrailingContext(matches, lastMatchIndex, pendingContext, opts.contextLines)
			}
			pendingContext = pendingContext[:0]
			lastMatchIndex = -1
			continue
		}
		var data rgMatchData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			continue
		}

		switch ev.Type {
		case "match":
			if len(matches) >= opts.limit {
				return matches, true, "matches", nil
			}
			m := buildMatch(data, opts.includeSubmatches)
			if len(pendingContext) > 0 {
				if lastMatchIndex >= 0 {
					attachBetweenMatches(matches, lastMatchIndex, &m, pendingContext, opts.contextLines)
				} else {
					m.Before = append(m.Before, pendingContext...)
				}
				pendingContext = pendingContext[:0]
			}
			matches = append(matches, m)
			lastMatchIndex = len(matches) - 1
			bytesUsed += len(m.Line)

			if bytesUsed >= MaxResultBytes {
				return matches, true, "bytes", nil
			}
		case "context":
			ctxLine := buildContextLine(data)
			pendingContext = append(pendingContext, ctxLine)
			bytesUsed += len(ctxLine.Line)
			if bytesUsed >= MaxResultBytes {
				return matches, true, "bytes", nil
			}
		default:
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return matches, false, "", fmt.Errorf("rg emitted a JSON line exceeding %d bytes", scannerLineCap)
		}
		return matches, false, "", err
	}
	return matches, false, "", nil
}

func attachBetweenMatches(matches []Match, lastMatchIndex int, current *Match, context []ContextLine, contextLines int) {
	previousLine := matches[lastMatchIndex].LineNumber
	for _, line := range context {
		attached := false
		if contextLines > 0 && line.LineNumber <= previousLine+contextLines {
			matches[lastMatchIndex].After = append(matches[lastMatchIndex].After, line)
			attached = true
		}
		if contextLines > 0 && line.LineNumber >= current.LineNumber-contextLines {
			current.Before = append(current.Before, line)
			attached = true
		}
		if !attached {
			if line.LineNumber < current.LineNumber {
				current.Before = append(current.Before, line)
			} else {
				matches[lastMatchIndex].After = append(matches[lastMatchIndex].After, line)
			}
		}
	}
}

func attachTrailingContext(matches []Match, lastMatchIndex int, context []ContextLine, contextLines int) {
	previousLine := matches[lastMatchIndex].LineNumber
	for _, line := range context {
		if contextLines == 0 || line.LineNumber <= previousLine+contextLines {
			matches[lastMatchIndex].After = append(matches[lastMatchIndex].After, line)
		}
	}
}

func buildMatch(d rgMatchData, includeSubmatches bool) Match {
	path := strings.TrimPrefix(d.Path.decode(), "./")
	line, truncated := truncateLine(strings.TrimRight(d.Lines.decode(), "\n"))

	subs := make([]Submatch, 0, len(d.Submatches))
	if includeSubmatches {
		for _, s := range d.Submatches {
			if truncated && s.End > len(line) {
				continue
			}
			if s.Start < 0 || s.End < s.Start || s.End > len(line) {
				continue
			}
			subs = append(subs, Submatch{
				Text:  s.Match.decode(),
				Start: s.Start,
				End:   s.End,
			})
		}
	}

	return Match{
		Path:          path,
		LineNumber:    d.LineNumber,
		Line:          line,
		LineTruncated: truncated,
		Submatches:    subs,
	}
}

func buildContextLine(d rgMatchData) ContextLine {
	line, truncated := truncateLine(strings.TrimRight(d.Lines.decode(), "\n"))
	return ContextLine{
		LineNumber:    d.LineNumber,
		Line:          line,
		LineTruncated: truncated,
	}
}

func truncateLine(line string) (string, bool) {
	if len(line) <= MaxLineBytes {
		return line, false
	}
	end := MaxLineBytes
	for steps := 0; end > 0 && steps < utf8.UTFMax-1; steps++ {
		if utf8.RuneStart(line[end]) {
			break
		}
		end--
	}
	return line[:end], true
}

type pathErr struct {
	category string
	message  string
}

func (e *pathErr) Error() string { return e.message }

func newPathErr(category, format string, a ...any) *pathErr {
	return &pathErr{category: category, message: fmt.Sprintf(format, a...)}
}

func resolveExisting(workspacePath, rel string) (string, *pathErr) {
	if rel == "" {
		return "", newPathErr(ErrCategoryValidation, "path is required")
	}
	if filepath.IsAbs(rel) {
		return "", newPathErr(ErrCategoryValidation,
			"path must be workspace-relative, got absolute %q", rel)
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
	if !isDescendant(workspacePath, resolved) {
		return "", newPathErr(ErrCategoryPathEscape, "path %q resolves outside workspace", rel)
	}
	return resolved, nil
}

func isDescendant(parent, resolved string) bool {
	parent = filepath.Clean(parent)
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(parent, resolved)
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

func rejectDuplicateTopLevelKeys(raw []byte) error {
	return jsoncompat.RejectDuplicateTopLevelKeys(raw)
}

func failedResult(category, message string, start time.Time) Result {
	return Result{
		Outcome:    result.OutcomeFailed,
		Matches:    []Match{},
		MatchCount: 0,
		DurationMS: time.Since(start).Milliseconds(),
		Error:      &ResultError{Category: category, Message: message},
	}
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func isExecNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}

func rgErrorMessage(stderr *cappedBuffer, waitErr error) string {
	const maxStderrBytes = 1024
	s := strings.TrimSpace(stderr.String())
	if s == "" {
		if waitErr != nil {
			return waitErr.Error()
		}
		return "rg exited with non-zero status"
	}
	if len(s) > maxStderrBytes {
		end := maxStderrBytes
		for steps := 0; end > 0 && steps < utf8.UTFMax-1; steps++ {
			if utf8.RuneStart(s[end]) {
				break
			}
			end--
		}
		s = s[:end] + "...(truncated)"
	}
	return s
}

type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		c.buf.Write(p[:remaining])
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string { return c.buf.String() }

var (
	_ interface {
		Run(context.Context, Args) Result
	} = (*Tool)(nil)
	_ tool.InvokableTool = (*Tool)(nil)
)
