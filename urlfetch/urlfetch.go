package urlfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/mattsp1290/eino-tools/internal/jsoncompat"
	"github.com/mattsp1290/eino-tools/result"
)

// Name is the model-facing tool name.
const Name = "url_fetch"

// Args is the parsed input shape for the tool.
type Args struct {
	URL string `json:"url"`
}

// Result is the structured envelope returned to the model.
type Result struct {
	Outcome result.Outcome  `json:"outcome"`
	Content string          `json:"content,omitempty"`
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
}

const (
	ErrCategoryValidation = "validation"
	ErrCategoryNotFound   = "not_found"
	ErrCategoryIO         = "io"
	ErrCategoryNetwork    = "network"
	ErrCategoryCanceled   = "canceled"
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
	case ErrCategoryNetwork, ErrCategoryUnknown:
		return true
	default:
		return false
	}
}

// Tool fetches URL content.
type Tool struct {
	httpClient *http.Client
}

// New constructs a Tool.
func New(opts ...Options) (*Tool, error) {
	options, err := resolveOptions(opts)
	if err != nil {
		return nil, err
	}
	return &Tool{httpClient: options.HTTPClient}, nil
}

func resolveOptions(opts []Options) (Options, error) {
	switch len(opts) {
	case 0:
		return (Options{}).withDefaults(), nil
	case 1:
		return opts[0].withDefaults(), nil
	default:
		return Options{}, fmt.Errorf("urlfetch: expected at most one Options value, got %d", len(opts))
	}
}

const schemaJSON = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "url": {
      "type": "string",
      "minLength": 1,
      "description": "URL to fetch. Supported schemes: file:// (local filesystem) and https://. Returns the raw text content of the resource."
    }
  },
  "required": ["url"]
}`

// Schema returns a fresh JSON Schema copy for urlfetch arguments.
func Schema() json.RawMessage {
	return bytes.Clone([]byte(schemaJSON))
}

// Run executes the tool with parsed args.
func (t *Tool) Run(ctx context.Context, args Args) Result {
	if err := ctx.Err(); err != nil {
		return failedResult(ErrCategoryCanceled, err.Error())
	}

	rawURL := strings.TrimSpace(args.URL)
	if rawURL == "" {
		return failedResult(ErrCategoryValidation, "url is required and must be non-empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return failedResult(ErrCategoryValidation, fmt.Sprintf("malformed URL: %s", err))
	}

	switch u.Scheme {
	case "file":
		return t.runFile(u)
	case "https":
		return t.runHTTPS(ctx, rawURL)
	case "http":
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("unsupported scheme %q; use https", u.Scheme))
	default:
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("unsupported scheme %q; use https or file://", u.Scheme))
	}
}

func (t *Tool) runFile(u *url.URL) Result {
	if u.Host != "" {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("file URL must not include a host; use file:///path (got host %q)", u.Host))
	}

	// gosec G304: caller supplies the path; no workspace containment is intended.
	// The tool is designed to load arbitrary local files (e.g. doc assets).
	content, err := os.ReadFile(u.Path) //nolint:gosec
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return failedResult(ErrCategoryNotFound,
				fmt.Sprintf("file not found: %s", u.Path))
		}
		return failedResult(ErrCategoryIO,
			fmt.Sprintf("read error: %s", err))
	}

	return Result{
		Outcome: result.OutcomeSucceeded,
		Content: string(content),
	}
}

func (t *Tool) runHTTPS(ctx context.Context, rawURL string) Result {
	// gosec G107: variable URL in HTTP request is the tool's explicit purpose.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil) //nolint:gosec
	if err != nil {
		return failedResult(ErrCategoryValidation,
			fmt.Sprintf("could not create request: %s", err))
	}

	resp, err := t.httpClient.Do(req) //nolint:gosec // G704: SSRF is the tool's explicit purpose; caller owns policy
	if err != nil {
		if ctx.Err() != nil {
			return failedResult(ErrCategoryCanceled, ctx.Err().Error())
		}
		return failedResult(ErrCategoryNetwork,
			fmt.Sprintf("request failed: %s", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return failedResult(ErrCategoryNetwork,
			fmt.Sprintf("read response body: %s", err))
	}

	if resp.StatusCode == http.StatusNotFound {
		return failedResult(ErrCategoryNotFound,
			fmt.Sprintf("HTTP 404: resource not found at %s", rawURL))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return failedResult(ErrCategoryNetwork,
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
	}

	return Result{
		Outcome: result.OutcomeSucceeded,
		Content: string(body),
	}
}

// Info returns the Eino ToolInfo for urlfetch.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(schemaJSON), js); err != nil {
		return nil, fmt.Errorf("urlfetch: parse tool schema: %w", err)
	}
	return &schema.ToolInfo{
		Name:        Name,
		Desc:        "Fetch the raw text content of a file:// or https:// URL and return it as a string. Supported schemes: file:// (local filesystem) and https://. Fails fast with a structured error if the resource does not exist or is not accessible. Does not follow redirects beyond the standard library's defaults. Does not parse HTML, strip CSS, or interpret JavaScript.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

// InvokableRun is the Eino tool entry point.
func (t *Tool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	args := Args{}
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return "", errors.New("urlfetch: arguments JSON is required")
	}
	if err := jsoncompat.RejectDuplicateTopLevelKeys([]byte(trimmed)); err != nil {
		return "", fmt.Errorf("urlfetch: parse arguments: %w", err)
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "", fmt.Errorf("urlfetch: parse arguments: %w", err)
	}
	res := t.Run(ctx, args)
	out, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("urlfetch: marshal result: %w", err)
	}
	return string(out), nil
}

func failedResult(category, message string) Result {
	return Result{
		Outcome: result.OutcomeFailed,
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
