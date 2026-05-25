# ADR 0004: Tracker beads adapter uses SDK, not os/exec

## Status

Accepted, with upstream blocker.

## Context

The existing local-symphony beads adapter shells out to the `bd` CLI. For
eino-tools, `tracker/beads` is intended to be a reusable adapter over
`github.com/mattsp1290/beads-go/beads`, not another command runner.

Preflight verification on 2026-05-25 found `github.com/mattsp1290/beads-go`
reachable at commit `a6b2c7db3f574cfb3dff6df303def27bd6482b35`, but with no
tags, no `go.mod`, and no Go SDK files. Bead `eino-tools-m5h` tracks publishing
or authorizing that SDK module and tag.

## Decision

`tracker/beads` must import the beads-go SDK once it is available and adapt the
SDK close operation to `tracker.CloseWriter`.

`tracker/beads` must not import `os/exec`, must not shell out to `bd`, and must
not parse CLI text/JSON directly. The real `bd` binary belongs only in
integration tests for the SDK-backed adapter.

## Consequences

The tracker/beads implementation remains blocked until beads-go exposes an
importable module and close API surface. That is preferable to shipping a second
CLI wrapper and then breaking consumers when the SDK appears.

Dependency hygiene checks must assert that only `tracker/beads` imports
beads-go and that fileops, result, search, shell, and tracker do not pull it
transitively.
