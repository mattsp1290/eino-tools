# urlfetch inventory

**Package:** `github.com/mattsp1290/eino-tools/urlfetch`  
**Tool name:** `url_fetch`

## Purpose

Fetches the raw text content of a `file://` or `https://` URL and returns it as a string.
Designed for loading local documentation assets (e.g. `~/docs/learning-output-style/index.html`)
or HTTPS endpoints for system-prompt injection.

## Files

- `doc.go`: package contract; notes intentional absence of workspace containment.
- `options.go`: `Options` struct with injectable `HTTPClient`; default 30-second timeout.
- `urlfetch.go`: `Args`, `Result`, `Tool`, `New`, `Run`, `Info`, `InvokableRun`, `Schema`.
- `urlfetch_test.go`: tests for all public surface.

## Public API

- `const Name = "url_fetch"`
- `type Args struct { URL string }`
- `type Result struct { Outcome, Content, Error, RawJSON }`
- `type ResultError struct { Category, Message string }`
- `func New(opts ...Options) (*Tool, error)`
- `func Schema() json.RawMessage`
- `func (t *Tool) Run(ctx, args) Result`
- `func (t *Tool) Info(ctx) (*schema.ToolInfo, error)`
- `func (t *Tool) InvokableRun(ctx, argsJSON, ...tool.Option) (string, error)`

## Error categories

| Category | When |
|----------|------|
| `validation` | Unsupported scheme, malformed URL, empty URL, file://host/path |
| `not_found` | `file://` path does not exist, HTTP 404 |
| `io` | `file://` read error other than not-found |
| `network` | HTTP request failure (connection refused, TLS error, non-404 HTTP error) |
| `canceled` | Context canceled before or during I/O |
| `unknown` | Unexpected errors |

## Retry policy

`network` and `unknown` → retryable. All others → not retryable.

## Design notes

- **No workspace containment.** Unlike fileops tools, urlfetch intentionally reads arbitrary
  paths. This is documented in `doc.go` so reviewers don't flag the absence of containment
  as an oversight.
- `file://host/path` (non-empty authority) is rejected at validation — `url.Parse` populates
  `u.Host` with the authority, which `os.ReadFile(u.Path)` would silently ignore.
- Default HTTP client has a 30-second timeout; override via `Options.HTTPClient`.
- All tests use `httptest.NewTLSServer` (not `NewServer`) because `http://` is rejected at
  validation — a plain-HTTP test server would never receive a request.

## Dependencies

None beyond the Go standard library and existing eino-tools packages.
