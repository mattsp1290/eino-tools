# ADR 0008: Workspace Filesystem Serialization

## Status

Accepted

## Context

`eino-agent` will eventually run concurrent sessions and may run independent
tool calls in parallel. The existing file operation helpers use
validate-then-use path containment with symlink resolution. That is adequate for
single-call-at-a-time agent loops, but concurrent filesystem calls against the
same workspace can reintroduce symlink-swap races.

The request also asked whether web search and LSP/diagnostics belong in
`eino-tools`.

## Decision

`eino-tools` does not add a package-level lock in this slice. Instead,
`eino-agent` must serialize all workspace filesystem tools per workspace root:

- `fileops`
- `glob`
- `search`
- `apply_patch`

Independent workspace roots may run concurrently. The granularity is the
canonical workspace root path, not individual files or packages.

`web_search` is out of scope for `eino-tools`. `eino-agent` should own the
model-facing schema and runtime policy because provider credentials, rate
limits, freshness policy, permissions, citations, and browsing safety are
runtime connector concerns.

LSP/diagnostics are also out of scope for `eino-tools` in this parity slice.
`eino-agent` should own the model-facing schema unless a future optional module
is created around explicit server lifecycle, indexing state, diagnostics
timing, cancellation, permissions, and line/column conventions.

## Consequences

- `eino-agent` has a clear contract: serialize filesystem tools per workspace
  root.
- `eino-tools` remains a set of reusable leaf tools without runtime session
  policy or long-lived server lifecycle.
- A future openat-style containment implementation can relax the serialization
  requirement without changing the model-facing schemas.
