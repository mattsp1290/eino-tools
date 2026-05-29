# ADR 0006: userinteract tool-local Outcome enum

## Status

Accepted.

## Context

`userinteract` needs to signal three states: `succeeded` (answer available),
`pending` (MCP mode, answer not yet collected), and `failed`. The shared
`result.Outcome` enum (ADR 0001) is explicitly closed: unknown values fail
validation, and adding a new outcome is a breaking change requiring a
minor-version bump.

`pending` is specific to `userinteract`'s MCP surface contract. No other tool
will ever return it. Adding it to `result.Outcome` would couple a general-purpose
enum to a single tool's I/O protocol — the kind of abstraction leak ADR 0001 was
designed to prevent.

## Decision

`userinteract` defines its own `Outcome` type with three values:

```go
type Outcome string

const (
    OutcomeSucceeded Outcome = "succeeded"
    OutcomePending   Outcome = "pending"
    OutcomeFailed    Outcome = "failed"
)
```

It does not import or use `result.Outcome`. The deviation from the "all tools use
`result.Outcome`" convention is noted explicitly in `doc.go`.

The alternative (`result.Outcome` + a `Pending bool` field) was rejected because
`outcome: "succeeded"` with `pending: true` and no `Answer` is semantically
incoherent. A single switch on `Outcome` in the consumer is cleaner than checking
two fields.

## Consequences

Consumers that handle tool results generically by outcome cannot treat
`userinteract.Outcome` and `result.Outcome` as the same type. Callers must import
the `userinteract` package directly and handle the `pending` outcome explicitly.
This is acceptable: the tool's surface-aware pending state is not a generic concept
and consumers are expected to handle it explicitly.

The `IsRetryable` method on `userinteract.Result` returns `false` for all outcomes,
including `unknown`. This diverges from `shell` and `urlfetch` (which return `true`
for `unknown`). The divergence is intentional: stdin I/O errors and validation errors
on a human-interaction tool are not transient failures that benefit from automated
retry. This is documented in `doc.go`.
