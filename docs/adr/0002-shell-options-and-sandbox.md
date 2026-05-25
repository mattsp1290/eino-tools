# ADR 0002: Shell options and sandbox boundary

## Status

Accepted.

## Context

The shell tool is extracted from local-symphony as an Eino-compatible command
execution tool. Its model-facing schema is intentionally small: `cmd` and
`timeout_seconds`. The existing behavior runs commands through a shell so
models can use normal command strings, pipes, redirects, and environment
expansion.

Sandboxing is a deployment/runtime concern. This package can enforce argument
shape, output caps, timeouts, and constructor configuration, but it cannot know
the caller's workspace, OS containment, network policy, or process isolation
requirements.

## Decision

Preserve the model-facing schema:

- `cmd`: required command string.
- `timeout_seconds`: optional per-call timeout.

Preserve intentional shell execution through `sh -lc` by default. This is part
of the tool contract: callers and prompts expect shell syntax, not argv arrays.

Expose constructor-level options for host-controlled behavior:

- `Env`: optional environment overlay or replacement, depending on the final
  local-symphony source inventory.
- `ShellBinary`: override for the shell executable; default `sh`.
- `OutputCapBytes`: maximum captured stdout/stderr bytes before truncation.

The package must document that sandboxing is caller-owned. Callers decide
working directory containment, filesystem mounts, network isolation, UID/GID,
resource limits, and whether the tool is available at all.

## Consequences

The shell package may import `os/exec`; that import is expected only in shell
and search packages. Fileops, result, tracker, and tracker/beads must not pull
shell transitively.

The package should fail closed on invalid constructor configuration, empty
commands, invalid timeouts, and context cancellation. It should not attempt to
invent a partial sandbox in library code.

Any future argv-mode shell variant needs a new design record because it changes
prompt semantics and the model-facing schema.
