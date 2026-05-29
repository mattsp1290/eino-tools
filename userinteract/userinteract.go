package userinteract

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
)

// Name is the model-facing tool name.
const Name = "user_interact"

// Surface identifies the runtime context in which the tool is operating.
type Surface string

const (
	// SurfaceCLI is for interactive terminal sessions where blocking on stdin
	// is safe.
	SurfaceCLI Surface = "cli"
	// SurfaceMCP is for MCP server contexts where blocking is fatal.
	SurfaceMCP Surface = "mcp"
)

// Outcome is userinteract's own discriminator. It is not result.Outcome.
// The "pending" outcome only exists in this tool; adding it to the shared
// result package would violate ADR 0001's closed-enum rule (see ADR 0006).
type Outcome string

const (
	// OutcomeSucceeded indicates the answer is available in Result.Answer.
	OutcomeSucceeded Outcome = "succeeded"
	// OutcomePending indicates MCP mode is awaiting the user's answer via a
	// follow-up call with Args.Answer populated.
	OutcomePending Outcome = "pending"
	// OutcomeFailed indicates the tool encountered an error.
	OutcomeFailed Outcome = "failed"
)

// Args is the parsed input shape for the tool.
type Args struct {
	Question string `json:"question"`
	// Answer is reserved for the agent loop. Do not populate this field — it
	// will be set by the loop after the user responds. When non-empty, the
	// tool returns this value as the answer immediately.
	Answer string `json:"answer,omitempty"`
}

// Result is the structured envelope returned to the model.
type Result struct {
	Outcome Outcome `json:"outcome"`
	Answer  string  `json:"answer,omitempty"`
	// Question is set only when Outcome == OutcomePending. It echoes
	// Args.Question so the MCP host can display it without tracking it
	// separately.
	Question string          `json:"question,omitempty"`
	Error    *ResultError    `json:"error,omitempty"`
	RawJSON  json.RawMessage `json:"-"`
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
	ErrCategoryIO         = "io"
	ErrCategoryUnknown    = "unknown"
)

// IsRetryable reports whether the agent loop should retry the same call.
// Returns false for all outcomes — see doc.go for rationale.
func (r Result) IsRetryable() bool {
	return false
}

// Tool asks the user a question and returns their answer.
type Tool struct {
	surface Surface
	stdin   io.Reader
	stderr  io.Writer
}

// New constructs a Tool for the given surface.
func New(surface Surface, opts ...Options) (*Tool, error) {
	if surface != SurfaceCLI && surface != SurfaceMCP {
		return nil, fmt.Errorf("userinteract: unknown surface %q; use SurfaceCLI or SurfaceMCP", surface)
	}
	options, err := resolveOptions(opts)
	if err != nil {
		return nil, err
	}

	t := &Tool{surface: surface}
	if surface == SurfaceCLI {
		t.stdin = options.Stdin
		t.stderr = options.Stderr
	}
	// MCP surface intentionally holds no stdin Reader — see doc.go.
	return t, nil
}

func resolveOptions(opts []Options) (Options, error) {
	switch len(opts) {
	case 0:
		return (Options{}).withDefaults(), nil
	case 1:
		return opts[0].withDefaults(), nil
	default:
		return Options{}, fmt.Errorf("userinteract: expected at most one Options value, got %d", len(opts))
	}
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "question": {
      "type": "string",
      "minLength": 1,
      "description": "The question to present to the user."
    },
    "answer": {
      "type": "string",
      "description": "Reserved for the agent loop. Do not populate this field — it will be set by the loop after the user responds. When non-empty, the tool returns this value as the answer immediately."
    }
  },
  "required": ["question"]
}`

// Schema returns a fresh JSON Schema copy for userinteract arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

const stdinLineCap = 1 << 20 // 1 MiB per line

// Run executes the tool with parsed args.
func (t *Tool) Run(_ context.Context, args Args) Result {
	if strings.TrimSpace(args.Question) == "" {
		return failedResult(ErrCategoryValidation, "question is required and must be non-empty")
	}

	// Precedence 1: answer already provided (both surfaces).
	if args.Answer != "" {
		return Result{
			Outcome: OutcomeSucceeded,
			Answer:  args.Answer,
		}
	}

	switch t.surface {
	case SurfaceCLI:
		return t.runCLI(args.Question)
	case SurfaceMCP:
		return Result{
			Outcome:  OutcomePending,
			Question: args.Question,
		}
	default:
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("unknown surface %q", t.surface))
	}
}

func (t *Tool) runCLI(question string) Result {
	if _, err := fmt.Fprintf(t.stderr, "%s\n> ", question); err != nil {
		return failedResult(ErrCategoryIO, fmt.Sprintf("write prompt: %s", err))
	}

	scanner := bufio.NewScanner(t.stdin)
	scanner.Buffer(make([]byte, stdinLineCap), stdinLineCap)

	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // blank line terminates input
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return failedResult(ErrCategoryIO, fmt.Sprintf("read stdin: %s", err))
	}

	return Result{
		Outcome: OutcomeSucceeded,
		Answer:  strings.TrimRight(strings.Join(lines, "\n"), "\n"),
	}
}

// Info returns the Eino ToolInfo for userinteract.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("userinteract: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Ask the user a question and return their answer as a string. In CLI mode, prints the question to stderr and reads a multi-line answer from stdin (terminated by a blank line or EOF). In MCP mode, returns immediately with a pending result — provide the user's answer in a follow-up call by populating the answer field. Does not format questions, validate answers, or offer multiple-choice options.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	args := Args{}
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("userinteract: arguments JSON is required")
	}
	if err := jsoncompat.RejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("userinteract: parse arguments: %w", err)
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("userinteract: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("userinteract: marshal result: %w", err)
	}
	return string(out), nil
}

func failedResult(category, message string) Result {
	return Result{
		Outcome: OutcomeFailed,
		Error: &ResultError{
			Category: category,
			Message:  message,
		},
	}
}

var (
	_ interface {
		Run(context.Context, Args) Result
	} = (*Tool)(nil)
	_ tool.InvokableTool = (*Tool)(nil)
)
