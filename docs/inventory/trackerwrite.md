# Trackerwrite extraction inventory

Source inspected: `/home/infra-admin/git/local-symphony/internal/worker/tools/trackerwrite`
and `/home/infra-admin/git/local-symphony/internal/tracker/tracker.go` on
2026-05-25.

## Files

- `internal/worker/tools/trackerwrite/doc.go`: package contract, v1 scope, and
  error-envelope rationale.
- `internal/worker/tools/trackerwrite/trackerwrite.go`: tool implementation,
  schema, duplicate-key guard, result envelope, close dispatch, and error
  mapping.
- `internal/worker/tools/trackerwrite/trackerwrite_test.go`: close-path,
  unsupported-op, schema, duplicate-key, concurrency, error mapping, and retry
  policy tests.
- `internal/tracker/tracker.go`: broader local-symphony tracker interfaces.

## Public Surface To Preserve

- Tool name: `Name = "tracker_write"`.
- Op constants:
  - `OpComment = "comment"`
  - `OpTransition = "transition"`
  - `OpClose = "close"`
  - `OpLinkPR = "link_pr"`
- `Op.Valid()` accepts exactly those four values.
- `Schema() json.RawMessage` returns a fresh copy.
- `New(writer)` rejects nil writers.
- `Tool.Run(ctx, Args) Result` returns a structured `Result` for all runtime
  outcomes.
- `Tool.Info(ctx)` returns Eino `*schema.ToolInfo`.
- `Tool.InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)`
  rejects empty, malformed, and duplicate top-level JSON before execution.

## V0.1 Interface Boundary

Local-symphony's `tracker.TrackerWriter` contains:

- `Comment(ctx, id, body string) error`
- `Transition(ctx, id string, toState core.IssueState) error`
- `Close(ctx, id, reason string) error`
- `LinkPR(ctx, id, prURL string) error`

`eino-tools/tracker` must not lift that whole surface for v0.1. Per ADR 0003,
lift only:

```go
type CloseWriter interface {
	Close(ctx context.Context, id, reason string) error
}
```

`trackerwrite.Args` can keep parsing compatibility fields for the schema, but
the extracted package should avoid exporting or depending on
local-symphony `core.IssueState`. Use a string field for `toState` until a
future ADR promotes issue states.

## Schema Contract

The schema is an object with `additionalProperties: false`.

Required:

- `op`
- `id`

Properties:

- `op`: enum `["comment", "transition", "close", "link_pr"]`.
- `id`: string, `minLength: 1`.
- `body`: optional string, `minLength: 1`.
- `toState`: optional string, `minLength: 1`.
- `reason`: optional string.
- `prURL`: optional string, `minLength: 1`.

The schema intentionally accepts all four ops. In v0.1, per-op required fields
for unsupported ops are not enforced by schema, because execution should return
the more actionable `unsupported_op` envelope. If comment, transition, or
link_pr start executing, the schema must gain discriminator-aware per-op
validation.

## Close Execution Path

Execution validation order:

1. Nil receiver -> failed result, `validation`.
2. Invalid or empty op -> failed result, `validation`.
3. Empty/whitespace `id` -> failed result, `validation`.
4. Pre-dispatch `ctx.Err()` -> failed result, `canceled` or `timeout`.
5. `op=close` calls `writer.Close(ctx, args.ID, args.Reason)`.
6. Successful close returns `outcome=succeeded`, echoes `op` and `id`.
7. Close error maps tracker categories into model-facing result categories.

`reason` is optional and forwarded as provided by the tool; the beads adapter is
responsible for any whitespace trimming noted by local-symphony comments.

## Unsupported-Op Behavior

For `comment`, `transition`, and `link_pr`, v0.1 must not call writer methods.
It returns:

- `outcome=failed`
- `error.category=unsupported_op`
- `op` and `id` echoed
- `error.op` set when `op` is non-empty

This behavior is part of compatibility with current local-symphony prompts and
tests.

## Result Envelope

`Result` fields:

- `Outcome`: replace `dispatcher.ToolOutcome` with `result.Outcome`.
- `Op`: echoed op.
- `ID`: echoed issue id.
- `Error`: optional `ResultError`.

`ResultError` fields:

- `Category`
- `Message`
- `Op`

Retryable result categories:

- `api_request`
- `rate_limited`
- `timeout`
- `unknown`

Non-retryable result categories:

- `validation`
- `unsupported_op`
- `unsupported`
- `not_found`
- `auth_failed`
- `conflict`
- `api_status`
- `canceled`

## Error Category Mapping

Preserve model-facing strings:

- `unsupported_op`
- `validation`
- `not_found`
- `auth_failed`
- `conflict`
- `api_request`
- `api_status`
- `rate_limited`
- `timeout`
- `unsupported`
- `unknown`
- `canceled`

Adapter errors currently map through `tracker.CategoryOf(err)`, with wrapped
`context.Canceled` and `context.DeadlineExceeded` overriding unknown tracker
classification to `canceled` or `timeout`.

## Coupling To Replace

- Replace `local-symphony/internal/dispatcher.ToolOutcome*` with
  `eino-tools/result.Outcome*`.
- Replace the local-symphony `tracker.TrackerWriter` dependency with
  `eino-tools/tracker.CloseWriter`.
- Do not import `local-symphony/internal/core`; represent `toState` as a string
  until state promotion is explicitly designed.
- Do not import local-symphony dispatcher, telemetry, auth, or beads adapter
  packages.

## Tests To Lift

The implementation work should lift or recreate tests for:

- Successful close forwards id and reason.
- Empty reason forwards as empty.
- Close idempotency behavior.
- Unsupported ops do not call writer and return `unsupported_op`.
- Invalid op, empty op, empty id, nil receiver.
- Pre-dispatch cancellation and deadline.
- Writer error mapping, including wrapped context errors.
- Invokable success serialization and unsupported-op serialization.
- Empty/malformed/duplicate top-level JSON errors.
- Schema validity, fresh schema slices, unknown-property rejection, required
  op/id validation, enum validation, and all four ops parsing.
- Stable `Name`.
- `Result.IsRetryable` policy.
