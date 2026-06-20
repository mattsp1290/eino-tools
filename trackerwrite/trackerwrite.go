package trackerwrite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
	"github.com/mattsp1290/eino-tools/result"
	"github.com/mattsp1290/eino-tools/tracker"
)

// Name is the tool's registered name.
const Name = "tracker_write"

// Op is the discriminator for tracker_write arguments.
type Op string

const (
	OpComment    Op = "comment"
	OpTransition Op = "transition"
	OpClose      Op = "close"
	OpLinkPR     Op = "link_pr"
)

// Valid reports whether o is one of the model-facing ops.
func (o Op) Valid() bool {
	switch o {
	case OpComment, OpTransition, OpClose, OpLinkPR:
		return true
	default:
		return false
	}
}

const allOpsLabel = "close, comment, link_pr, transition"

// Args is the parsed tracker_write input shape.
type Args struct {
	Op      Op     `json:"op"`
	ID      string `json:"id"`
	Body    string `json:"body,omitempty"`
	ToState string `json:"toState,omitempty"`
	Reason  string `json:"reason,omitempty"`
	PRURL   string `json:"prURL,omitempty"`
}

// Result is the structured envelope returned to the model.
type Result struct {
	Outcome result.Outcome  `json:"outcome"`
	Op      Op              `json:"op,omitempty"`
	ID      string          `json:"id,omitempty"`
	Error   *ResultError    `json:"error,omitempty"`
	RawJSON json.RawMessage `json:"-"`
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
	Op       string `json:"op,omitempty"`
}

// IsRetryable reports whether the agent loop should retry the same call.
func (r Result) IsRetryable() bool {
	if r.Outcome == result.OutcomeSucceeded {
		return false
	}
	if r.Error == nil {
		return false
	}
	switch r.Error.Category {
	case ErrCategoryAPIRequest,
		ErrCategoryRateLimited,
		ErrCategoryTimeout,
		ErrCategoryUnknown:
		return true
	default:
		return false
	}
}

const (
	ErrCategoryUnsupportedOp = "unsupported_op"
	ErrCategoryValidation    = "validation"
	ErrCategoryNotFound      = "not_found"
	ErrCategoryAuthFailed    = "auth_failed"
	ErrCategoryConflict      = "conflict"
	ErrCategoryAPIRequest    = "api_request"
	ErrCategoryAPIStatus     = "api_status"
	ErrCategoryRateLimited   = "rate_limited"
	ErrCategoryTimeout       = "timeout"
	ErrCategoryUnsupported   = "unsupported"
	ErrCategoryUnknown       = "unknown"
	ErrCategoryCanceled      = "canceled"
)

// Tool is the v0.1 tracker_write tool.
type Tool struct {
	writer tracker.CloseWriter
	// transitionWriter is non-nil only when writer also implements
	// tracker.TransitionWriter; when it is set it is the same value as writer.
	// nil means op=transition returns unsupported_op.
	transitionWriter tracker.TransitionWriter
}

// New constructs a Tool.
func New(w tracker.CloseWriter) (*Tool, error) {
	if w == nil {
		return nil, errors.New("tracker_write: CloseWriter is required")
	}
	t := &Tool{writer: w}
	// Implementing tracker.TransitionWriter is the opt-in mechanism for
	// exposing op=transition to the model: a CloseWriter that grows a matching
	// Transition method silently gains the transition surface, so don't add one
	// casually. transitionWriter stays nil for close-only writers.
	t.transitionWriter, _ = w.(tracker.TransitionWriter)
	return t, nil
}

// supportedOps returns the comma-separated set of ops this tool can actually
// execute, so unsupported_op messages never claim a capability the configured
// writer lacks (or omit one it has).
func (t *Tool) supportedOps() string {
	if t.transitionWriter != nil {
		return string(OpClose) + ", " + string(OpTransition)
	}
	return string(OpClose)
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "op": {
      "type": "string",
      "enum": ["comment", "transition", "close", "link_pr"],
      "description": "Discriminator. v1 implements 'close' and optionally 'transition' when the configured writer supports it; other ops return tool_failed{error.category=unsupported_op}."
    },
    "id": {
      "type": "string",
      "minLength": 1,
      "description": "Tracker issue identifier."
    },
    "body": {
      "type": "string",
      "minLength": 1,
      "description": "Comment body. Required for op=comment (post-v1)."
    },
    "toState": {
      "type": "string",
      "minLength": 1,
      "description": "Target issue state for op=transition. If the configured writer does not support transitions, op=transition returns unsupported_op regardless of this value."
    },
    "reason": {
      "type": "string",
      "description": "Optional close reason."
    },
    "prURL": {
      "type": "string",
      "minLength": 1,
      "description": "Pull-request URL. Required for op=link_pr (post-v1)."
    }
  },
  "required": ["op", "id"]
}`

// Schema returns a fresh JSON Schema copy for tracker_write arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

// Run executes tracker_write against parsed args.
func (t *Tool) Run(ctx context.Context, args Args) Result {
	if t == nil {
		return failedResult(args, ErrCategoryValidation, "tracker_write tool is not configured for this run")
	}
	if !args.Op.Valid() {
		opLabel := string(args.Op)
		if opLabel == "" {
			opLabel = "(empty)"
		}
		return failedResult(args, ErrCategoryValidation,
			fmt.Sprintf("op %q is not one of: %s", opLabel, allOpsLabel))
	}
	if strings.TrimSpace(args.ID) == "" {
		return failedResult(args, ErrCategoryValidation, "id is required and must be non-empty")
	}
	if err := ctx.Err(); err != nil {
		return failedResult(args, contextErrCategory(err), err.Error())
	}

	switch args.Op {
	case OpClose:
		if err := t.writer.Close(ctx, args.ID, args.Reason); err != nil {
			return failedFromWriterErr(args, err)
		}
		return succeededResult(args)
	case OpTransition:
		// Capability is checked before shape: a close-only writer can never
		// perform a transition, so report unsupported_op (terminal) rather than
		// asking the model to retry with toState.
		if t.transitionWriter == nil {
			return failedResult(args, ErrCategoryUnsupportedOp,
				fmt.Sprintf("op %q is not supported by the configured writer; supported ops: [%s]",
					args.Op, t.supportedOps()))
		}
		toState := strings.TrimSpace(args.ToState)
		if toState == "" {
			return failedResult(args, ErrCategoryValidation, "toState is required and must be non-empty for op transition")
		}
		if err := t.transitionWriter.Transition(ctx, args.ID, toState); err != nil {
			return failedFromWriterErr(args, err)
		}
		return succeededResult(args)
	case OpComment, OpLinkPR:
		return failedResult(args, ErrCategoryUnsupportedOp,
			fmt.Sprintf("op %q is defined but not implemented in v1; supported ops: [%s]",
				args.Op, t.supportedOps()))
	default:
		return failedResult(args, ErrCategoryValidation,
			fmt.Sprintf("op %q has no dispatch (v1 routing bug)", args.Op))
	}
}

// Info returns the Eino ToolInfo for tracker_write.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("tracker_write: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Mutate the issue tracker. v1 implements op=close and can implement op=transition when the configured writer supports transitions. Unsupported ops return tool_failed{error.category=unsupported_op}.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	args := Args{}
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("tracker_write: arguments JSON is required")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("tracker_write: parse arguments: %w", err)
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("tracker_write: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("tracker_write: marshal result: %w", err)
	}
	return string(out), nil
}

func rejectDuplicateTopLevelKeys(raw []byte) error {
	return jsoncompat.RejectDuplicateTopLevelKeys(raw)
}

func succeededResult(args Args) Result {
	return Result{
		Outcome: result.OutcomeSucceeded,
		Op:      args.Op,
		ID:      args.ID,
	}
}

func failedResult(args Args, category, message string) Result {
	r := Result{
		Outcome: result.OutcomeFailed,
		Op:      args.Op,
		ID:      args.ID,
		Error: &ResultError{
			Category: category,
			Message:  message,
		},
	}
	if args.Op != "" {
		r.Error.Op = string(args.Op)
	}
	return r
}

func failedFromWriterErr(args Args, err error) Result {
	return failedResult(args, contextErrCategory(err), err.Error())
}

func contextErrCategory(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return ErrCategoryTimeout
	case errors.Is(err, context.Canceled):
		return ErrCategoryCanceled
	default:
		return ErrCategoryUnknown
	}
}

var (
	_ interface {
		Run(context.Context, Args) Result
	} = (*Tool)(nil)
	_ tool.InvokableTool = (*Tool)(nil)
)
