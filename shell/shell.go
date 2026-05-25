//go:build unix

package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
	"github.com/mattsp1290/eino-tools/result"
)

// Name is the model-facing tool name.
const Name = "shell"

const (
	// DefaultTimeoutSeconds is applied when timeout_seconds is omitted or zero.
	DefaultTimeoutSeconds = 60

	// MaxTimeoutSeconds is the maximum accepted per-call timeout.
	MaxTimeoutSeconds = 600

	waitDelayAfterCancel = 5 * time.Second
)

// Args is the parsed input shape for the tool.
type Args struct {
	Cmd            string `json:"cmd"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// Result is the structured envelope returned to the model.
type Result struct {
	Outcome         result.Outcome  `json:"outcome"`
	ExitCode        int             `json:"exit_code"`
	Stdout          string          `json:"stdout"`
	Stderr          string          `json:"stderr"`
	StdoutTruncated bool            `json:"stdout_truncated,omitempty"`
	StderrTruncated bool            `json:"stderr_truncated,omitempty"`
	DurationMS      int64           `json:"duration_ms"`
	TimedOut        bool            `json:"timed_out,omitempty"`
	Error           *ResultError    `json:"error,omitempty"`
	RawJSON         json.RawMessage `json:"-"`
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

// ResultError is the structured failure envelope nested inside Result.
type ResultError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

const (
	ErrCategoryValidation = "validation"
	ErrCategoryTimeout    = "timeout"
	ErrCategoryCanceled   = "canceled"
	ErrCategoryExecFailed = "exec_failed"
	ErrCategoryUnknown    = "unknown"
)

// IsRetryable reports whether the agent loop should retry the same call.
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

// Tool executes shell commands in a configured workspace directory.
type Tool struct {
	workspacePath  string
	shellBinary    string
	env            []string
	outputCapBytes int
}

// New constructs a Tool from an absolute workspace path.
func New(workspacePath string, opts ...Options) (*Tool, error) {
	if workspacePath == "" {
		return nil, errors.New("shell: workspace path is required")
	}
	if !filepath.IsAbs(workspacePath) {
		return nil, fmt.Errorf("shell: workspace path must be absolute, got %q", workspacePath)
	}
	options, err := resolveOptions(opts)
	if err != nil {
		return nil, err
	}
	return &Tool{
		workspacePath:  workspacePath,
		shellBinary:    options.ShellBinary,
		env:            options.Env,
		outputCapBytes: options.OutputCapBytes,
	}, nil
}

func resolveOptions(opts []Options) (Options, error) {
	switch len(opts) {
	case 0:
		return (Options{}).withDefaults(), nil
	case 1:
		o := opts[0]
		if o.OutputCapBytes < 0 {
			return Options{}, fmt.Errorf("shell: output cap bytes must be non-negative, got %d", o.OutputCapBytes)
		}
		return o.withDefaults(), nil
	default:
		return Options{}, fmt.Errorf("shell: expected at most one Options value, got %d", len(opts))
	}
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "cmd": {
      "type": "string",
      "minLength": 1,
      "description": "Shell command body. Run as 'sh -lc <cmd>' in the agent's workspace cwd."
    },
    "timeout_seconds": {
      "type": "integer",
      "minimum": 0,
      "maximum": 600,
      "description": "Per-call timeout in seconds. 0 or omitted applies the default (60s). Cap is 600s."
    }
  },
  "required": ["cmd"]
}`

// Schema returns a fresh JSON Schema copy for shell arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

// Run executes the tool with parsed args.
func (t *Tool) Run(ctx context.Context, args Args) Result {
	if t == nil {
		return failedResult(ErrCategoryValidation, "shell tool is not configured for this run")
	}
	if strings.TrimSpace(args.Cmd) == "" {
		return failedResult(ErrCategoryValidation, "cmd is required and must be non-empty")
	}
	if args.TimeoutSeconds < 0 {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("timeout_seconds must be non-negative, got %d", args.TimeoutSeconds))
	}
	if args.TimeoutSeconds > MaxTimeoutSeconds {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("timeout_seconds %d exceeds maximum %d", args.TimeoutSeconds, MaxTimeoutSeconds))
	}
	if err := ctx.Err(); err != nil {
		return failedResult(ErrCategoryCanceled, err.Error())
	}

	timeout := time.Duration(args.TimeoutSeconds) * time.Second
	if args.TimeoutSeconds == 0 {
		timeout = time.Duration(DefaultTimeoutSeconds) * time.Second
	}

	parentCtx := ctx
	runCtx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	stdout := newCappedBuffer(t.outputCapBytes)
	stderr := newCappedBuffer(t.outputCapBytes)

	// gosec G204: model-supplied command execution is intentional. The
	// caller/container owns workspace containment, sandboxing, and egress
	// policy; this tool only sets cwd and the documented shell boundary.
	cmd := exec.CommandContext(runCtx, t.shellBinary, "-lc", args.Cmd) //nolint:gosec
	cmd.Dir = t.workspacePath
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Env = t.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = waitDelayAfterCancel

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	res := Result{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
		DurationMS:      elapsed.Milliseconds(),
	}

	switch {
	case runErr == nil:
		res.Outcome = result.OutcomeSucceeded
		res.ExitCode = 0
		return res
	case parentCtx.Err() != nil:
		res.Outcome = result.OutcomeFailed
		res.ExitCode = exitCodeOf(runErr)
		res.Error = &ResultError{
			Category: ErrCategoryCanceled,
			Message: fmt.Sprintf("command canceled by parent context: %s (%s)",
				parentCtx.Err(), runErr),
		}
		return res
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		res.Outcome = result.OutcomeFailed
		res.ExitCode = exitCodeOf(runErr)
		res.TimedOut = true
		res.Error = &ResultError{
			Category: ErrCategoryTimeout,
			Message: fmt.Sprintf("command exceeded %ds timeout: %s",
				int(timeout.Seconds()), runErr),
		}
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.Outcome = result.OutcomeSucceeded
		res.ExitCode = exitErr.ExitCode()
		return res
	}

	res.Outcome = result.OutcomeFailed
	res.ExitCode = -1
	res.Error = &ResultError{
		Category: ErrCategoryExecFailed,
		Message:  runErr.Error(),
	}
	return res
}

// Info returns the Eino ToolInfo for shell.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("shell: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Run a shell command via 'sh -lc <cmd>' in the agent's workspace cwd. Captures stdout, stderr, exit code, and duration. Per-call timeout defaults to 60s and is capped at 600s. Stdout/stderr are capped at 256 KiB each; oversize output sets truncated=true.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	args := Args{}
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("shell: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("shell: parse arguments: %w", err)
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("shell: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("shell: marshal result: %w", err)
	}
	return string(out), nil
}

func rejectDuplicateTopLevelKeys(raw []byte) error {
	return jsoncompat.RejectDuplicateTopLevelKeys(raw)
}

func failedResult(category, message string) Result {
	return Result{
		Outcome:  result.OutcomeFailed,
		ExitCode: -1,
		Error: &ResultError{
			Category: category,
			Message:  message,
		},
	}
}

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
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
		c.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		end := remaining
		for steps := 0; end > 0 && steps < utf8.UTFMax-1; steps++ {
			if utf8.RuneStart(p[end]) {
				break
			}
			end--
		}
		if end > 0 {
			c.buf.Write(p[:end])
		}
		c.truncated = true
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string { return c.buf.String() }

func (c *cappedBuffer) Truncated() bool { return c.truncated }

var (
	_ interface {
		Run(context.Context, Args) Result
	} = (*Tool)(nil)
	_ tool.InvokableTool = (*Tool)(nil)
)
