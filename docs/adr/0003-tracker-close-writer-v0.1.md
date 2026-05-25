# ADR 0003: Tracker CloseWriter v0.1 scope

## Status

Accepted.

## Context

The local-symphony tracker abstraction has a broader reader/writer surface than
eino-tools v0.1 needs. The extracted `tracker_write` tool only has one real v1
execution path: close an issue with an optional reason. Other model-facing ops
exist in the current schema for compatibility, but they return
`unsupported_op`.

Publishing the full local-symphony tracker triad would freeze consumer-specific
types into this shared module too early.

## Decision

Define the public v0.1 tracker interface as close-only:

```go
type CloseWriter interface {
	Close(ctx context.Context, id, reason string) error
}
```

Defer these surfaces from v0.1:

- Comment writing.
- Issue transition/state mutation.
- LinkPR.
- Public `IssueState` enum.
- Tracker reader APIs.

The `tracker_write` tool may continue to parse the current local-symphony
fields needed to preserve schema compatibility, but unsupported operations must
return the existing `unsupported_op` result envelope.

## Consequences

The close-only interface keeps `eino-tools/tracker` small and testable. It also
lets local-symphony keep its richer tracker abstractions internally while thin
wrappers adapt to `CloseWriter`.

Because this module is pre-v1.0, breaking changes are allowed between minor
versions. Promoting comment, transition, LinkPR, or state types requires a
minor-version bump and changelog migration note.
