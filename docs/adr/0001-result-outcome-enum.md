# ADR 0001: Result Outcome enum

## Status

Accepted.

## Context

The extracted tools currently depend on
`local-symphony/internal/dispatcher.ToolOutcome` for the model-facing
`outcome` field in per-tool JSON result envelopes. That import cannot move
into `github.com/mattsp1290/eino-tools`: it would couple a reusable tool kit to
local-symphony's dispatcher package and make every consumer inherit dispatcher
types it does not otherwise need.

The stable part worth sharing is the outcome discriminator, not a universal
result envelope. Each tool keeps its own result struct, error categories, retry
rules, and forward-compatible payload shape.

## Decision

Create package `result` with:

- `type Outcome string`
- `const OutcomeSucceeded Outcome = "succeeded"`
- `const OutcomeFailed Outcome = "failed"`
- `const OutcomeTimedOut Outcome = "timed_out"`
- `const OutcomeRejected Outcome = "rejected"`
- `func (o Outcome) Valid() bool`

Tool packages use `result.Outcome` for their `outcome` JSON fields. They do not
import `local-symphony/internal/dispatcher`.

The enum is closed for validation in v0.1. Unknown values fail validation and
must not silently pass through as successful tool results. Adding a new outcome
is a breaking change for consumers that switch exhaustively on outcomes, so it
requires a minor-version bump while the module is pre-v1 and a changelog entry.

## Consequences

The result package remains intentionally small. It does not define a shared
tool result envelope, shared error category enum, or shared retry policy.

During local-symphony adoption, use a thin shim:

```go
type ToolOutcome = result.Outcome

const (
	ToolOutcomeSucceeded = result.OutcomeSucceeded
	ToolOutcomeFailed    = result.OutcomeFailed
	ToolOutcomeTimedOut  = result.OutcomeTimedOut
	ToolOutcomeRejected  = result.OutcomeRejected
)
```

If local-symphony needs to keep a distinct dispatcher type for one release,
the wrapper layer must convert explicitly between the two string enums. Either
approach keeps extracted packages independent from dispatcher internals.

Per-tool result structs still own their JSON compatibility contracts. Adding a
typed field to a tool result can be non-breaking when the tool documents and
tests unknown-field preservation; renaming or retyping existing fields remains
breaking.
