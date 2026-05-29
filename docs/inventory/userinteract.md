# userinteract inventory

**Package:** `github.com/mattsp1290/eino-tools/userinteract`  
**Tool name:** `user_interact`

## Purpose

Asks the user a question and returns their typed answer. Must work correctly in
two surfaces without blocking the MCP server thread:

- **CLI mode:** prints the question to stderr, blocks reading a multi-line answer
  from stdin (terminated by a blank line or EOF).
- **MCP mode:** returns immediately with a `pending` result containing the question
  text. The caller is responsible for collecting the user's answer out of band and
  providing it in a follow-up call with `Args.Answer` populated.

## Files

- `doc.go`: package contract; documents Surface semantics, Outcome divergence from
  `result.Outcome`, IsRetryable divergence, and the single-outstanding-question assumption.
- `options.go`: `Options` struct with injectable `Stdin` and `Stderr`; defaults to
  `os.Stdin`/`os.Stderr`.
- `userinteract.go`: `Surface`, `Outcome`, `Args`, `Result`, `Tool`, `New`, `Run`,
  `Info`, `InvokableRun`, `Schema`.
- `userinteract_test.go`: tests for all public surface, including the panicking-reader
  MCP test.

## Public API

- `const Name = "user_interact"`
- `type Surface string` — `SurfaceCLI`, `SurfaceMCP`
- `type Outcome string` — `OutcomeSucceeded`, `OutcomePending`, `OutcomeFailed`
- `type Args struct { Question, Answer string }`
- `type Result struct { Outcome, Answer, Question, Error, RawJSON }`
- `type ResultError struct { Category, Message string }`
- `func New(surface Surface, opts ...Options) (*Tool, error)`
- `func Schema() json.RawMessage`
- `func (t *Tool) Run(ctx, args) Result`
- `func (t *Tool) Info(ctx) (*schema.ToolInfo, error)`
- `func (t *Tool) InvokableRun(ctx, argsJSON, ...tool.Option) (string, error)`

## Error categories

| Category | When |
|----------|------|
| `validation` | Empty question, unknown surface value |
| `io` | Stdin read error in CLI mode |
| `unknown` | Unexpected errors |

## Retry policy

All outcomes → not retryable. This is an intentional divergence from `shell` and
`urlfetch`. Human-interaction I/O errors are not transient failures that benefit
from automated retry.

## Design notes

- **Tool-local Outcome enum.** `userinteract.Outcome` is distinct from `result.Outcome`
  because `pending` is specific to this tool's MCP contract (ADR 0006).
- **Stateless MCP.** The tool holds no cross-call state. The agent loop owns the
  pending→answer round-trip: persist the question, collect the human's response
  out of band, call the tool again with `Args.Answer` populated.
- **Injection at construction.** `stdin`/`stderr` are injected at `New()` (not
  per-call). An MCP-surface instance holds no Reader — the capability is absent
  rather than conditionally skipped.
- **`answer` is agent-loop-owned.** The `answer` field in Args exists as a transport
  field for the loop to inject the collected human response. It must not be
  described as something the model should populate.
- **Single-outstanding-question assumption.** The tool performs no pairing
  validation between a pending question and a subsequent answer. Concurrent questions
  require the loop to manage correlation externally.
- **1 MiB line cap.** The CLI scanner buffer is 1 MiB per line.

## Dependencies

None beyond the Go standard library and existing eino-tools packages.
