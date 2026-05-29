# Tool Spec: `urlfetch`

**Package:** `github.com/mattsp1290/eino-tools/urlfetch`  
**Status:** No open design questions — ready to implement on engineer confirmation  
**Dependencies introduced:** none (all stdlib)

---

## What it does

Fetches the raw text content of a `file://` or `https://` URL and returns it as a string. Needed by the planner agent to load a local HTML file (`file:///Users/punk1290/docs/learning-output-style/index.html`) for system-prompt injection.

---

## Files to create

```
urlfetch/
├── doc.go
├── options.go
├── urlfetch.go
└── urlfetch_test.go
```

`docs/inventory/urlfetch.md` and a README entry are also required (see `00-overview.md` boilerplate checklist).

---

## Args

```go
type Args struct {
    URL string `json:"url"`
}
```

- `url` is required, must be non-empty.
- Supported schemes: `file` and `https`. Reject others (including plain `http`) at validation with `ErrCategoryValidation`. The validation error message must name the rejected scheme and tell the caller to use `https://` (e.g. `unsupported scheme "http"; use https`) so a model retrying blind doesn't loop.

### JSON Schema

```json
{
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
}
```

---

## Result

```go
type Result struct {
    Outcome  result.Outcome  `json:"outcome"`
    Content  string          `json:"content,omitempty"`
    Error    *ResultError    `json:"error,omitempty"`
    RawJSON  json.RawMessage `json:"-"`
}
```

`Content` holds the raw bytes decoded as UTF-8. On failure, `Content` is empty and `Error` is populated.

`urlfetch` uses the shared `result.Outcome` enum — no new outcomes are needed. The existing `succeeded`/`failed` pair is sufficient.

### ResultError

```go
type ResultError struct {
    Category string `json:"category"`
    Message  string `json:"message"`
}
```

### Error categories

| Category | When |
|----------|------|
| `validation` | Unsupported scheme, malformed URL, empty URL |
| `not_found` | `file://` path does not exist, HTTP 404 |
| `io` | `file://` read error |
| `network` | HTTP request failure (connection refused, TLS error, non-404 HTTP error) |
| `unknown` | Unexpected errors that don't fit the above |

### IsRetryable

- `network` and `unknown` → `true` (transient failures)
- All others → `false`

---

## Tool struct and constructor

```go
type Tool struct {
    httpClient *http.Client
}

func New(opts ...Options) (*Tool, error) { ... }
```

No workspace path is required — URLs are absolute by definition. `file://` URLs intentionally reach outside any workspace: the canonical use case reads a doc from `~/docs/`. This is a deliberate contrast with the fileops tools (which enforce workspace containment); call it out in `doc.go` and the inventory entry so reviewers don't flag the absence as a mistake.

`file://host/path` (non-empty authority component) must be rejected with `ErrCategoryValidation` — `url.Parse` populates `u.Host` with the authority and `os.ReadFile(u.Path)` would silently ignore it. The rejection message should name what was wrong (e.g. `file URL must not include a host; use file:///path`).

### Options

```go
type Options struct {
    // HTTPClient overrides the default HTTP client. Useful in tests.
    // If nil, a client with a 30-second timeout is used.
    HTTPClient *http.Client
}
```

The default client must have an explicit timeout — `net/http`'s zero-value client hangs forever. Recommended default: 30 seconds.

```go
var defaultClient = &http.Client{Timeout: 30 * time.Second}
```

The injectable client is what makes HTTPS tests practical — see test cases for the correct pattern.

`resolveOptions` must mirror shell.go lines 129-142 exactly: accept 0 or 1 Options value, return an error if more than one is passed. This is the consistent cross-tool contract.

---

## Implementation notes

### Context propagation

`Run` must check `ctx.Err()` at the very top before doing any I/O (mirrors shell.go:184). For HTTPS, use `http.NewRequestWithContext` — not `client.Get`, which ignores `ctx` and is uncancelable:

```go
if err := ctx.Err(); err != nil {
    return failedResult(ErrCategoryCanceled, err.Error())
}
```

Add `ErrCategoryCanceled = "canceled"` to the error categories. `IsRetryable` returns false for canceled.

### `file://` handling

Parse with `url.Parse`. Reject non-empty `u.Host` with `ErrCategoryValidation` (see constructor section). Then call `os.ReadFile(u.Path)`. No containment check — by design.

Distinguish `not_found` from `io` using `errors.Is(err, fs.ErrNotExist)` — not the deprecated `os.IsNotExist`. The `errorlint` linter (enabled in `.golangci.yml`) requires `errors.Is`.

Annotate the `os.ReadFile` call (gosec G304: variable file path):

```go
// gosec G304: caller supplies the path; no workspace containment is intended.
// The tool is designed to load arbitrary local files (e.g. doc assets).
content, err := os.ReadFile(u.Path) //nolint:gosec
```

### `https://` handling

Use `http.NewRequestWithContext` so the call respects `ctx` cancellation. The G107 annotation belongs on the request-construction line where `rawURL` is consumed:

```go
// gosec G107: variable URL in HTTP request is the tool's explicit purpose.
req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil) //nolint:gosec
if err != nil { ... }
resp, err := t.httpClient.Do(req)
```

Always call `resp.Body.Close()` (defer after checking err).

Read `resp.Body` with `io.ReadAll`. Treat any non-2xx status as `ErrCategoryNetwork` or `ErrCategoryNotFound` (404 specifically). Return `resp.StatusCode` in the error message so the model can see it.

### Size cap (consideration, not requirement)

The request says "raw bytes → string is sufficient" and doesn't require a cap. However, loading a multi-megabyte file into a model context is rarely useful. Consider adding an optional `MaxBytes int` in Options (defaulting to uncapped or 1 MiB) to mirror shell's `OutputCapBytes`. Mark `Content` truncated if so. **This is optional** — implement only if the engineer wants it; the base spec does not require it.

---

## Test cases

All tests use `t.Parallel()` and table-driven patterns.

```
TestNew
  - nil opts succeeds (default client populated)
  - explicit http client preserved

TestTool_Run_FileScheme
  - existing file returns succeeded + content
  - non-existent path returns not_found
  - directory path (not a file) returns io or not_found
  - file:// with percent-encoded path decodes correctly

TestTool_Run_HTTPSScheme
  - 200 response returns succeeded + body content
  - 404 response returns not_found
  - 500 response returns network error
  - connection refused returns network error
  (use httptest.NewTLSServer — NOT httptest.NewServer, which serves http:// and
   would be rejected at validation before the request is made; inject srv.Client()
   via Options.HTTPClient so TLS cert verification passes against the test server)

TestTool_Run_Validation
  - empty URL returns validation error
  - http:// scheme rejected (validation); error message names scheme and suggests https://
  - ftp:// scheme rejected (validation)
  - malformed URL rejected (validation)
  - file://host/path rejected (validation; non-empty host)
  - canceled ctx returns canceled error before any I/O

TestTool_InvokableRun
  - empty JSON returns error
  - duplicate keys rejected
  - valid JSON round-trips through Run

TestResult_UnmarshalJSON
  - RawJSON preserved after unmarshal

TestSchema
  - Schema() returns valid JSON
  - required fields present
  - no additionalProperties

TestIsRetryable
  - succeeded → false
  - not_found → false
  - validation → false
  - canceled → false
  - network → true
  - unknown → true
```

---

## Model-facing tool description (for Info())

> Fetch the raw text content of a `file://` or `https://` URL and return it as a string. Supported schemes: `file://` (local filesystem) and `https://`. Fails fast with a structured error if the resource does not exist or is not accessible. Does not follow redirects beyond the standard library's defaults. Does not parse HTML, strip CSS, or interpret JavaScript.

---

## gosec suppressions summary

| Line | Rule | Justification |
|------|------|---------------|
| `os.ReadFile(u.Path)` | G304 | Caller supplies path; loading arbitrary local files is the tool's purpose |
| `http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)` | G107 | Variable URL in HTTP request is the tool's explicit purpose |

---

## New imports required in go.mod

None. All dependencies are in the Go standard library (`net/http`, `net/url`, `os`, `io`).
