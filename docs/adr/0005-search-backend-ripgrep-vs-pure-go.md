# ADR 0005: Search backend stays ripgrep for v0.1

## Status

Accepted.

## Context

The current local-symphony search tool shells out to ripgrep with JSON output.
That behavior is already reflected in prompts, result parsing, truncation
semantics, and tests. A pure-Go search backend could reduce external binary
requirements, but it would also require re-specifying regex behavior, ignore
rules, binary-file handling, JSON event shape, and prompt guidance.

## Decision

For v0.1, preserve the ripgrep backend and current `rg --json -e` behavior.

The search package may import `os/exec`. It must keep the existing
workspace-rooted path checks, result schema, output limits, and JSON event
parsing semantics from local-symphony.

A future pure-Go rewrite requires a separate design record that covers:

- Regex compatibility expectations.
- Directory traversal and ignore-file semantics.
- Binary-file handling.
- Result JSON compatibility and migration.
- Prompt/schema changes.
- Performance and output-cap comparisons against ripgrep.

## Consequences

Consumers of `search` need `rg` available at runtime for v0.1. The README and
tool docs should state that dependency plainly.

Dependency hygiene should allow `os/exec` for search and shell only. Fileops,
result, tracker, and tracker/beads must not import search transitively.
