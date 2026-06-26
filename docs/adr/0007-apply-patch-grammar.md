# ADR 0007: Apply Patch Grammar

## Status

Accepted

## Context

Coding agents need coordinated multi-file edits without shelling out or issuing
many independent write/edit calls. The request from `eino-agent` asked for a
Codex-style patch grammar with add, update, delete, and move support, plus
preflight and per-file summaries.

## Decision

`eino-tools` exposes package `applypatch` with model-facing tool name
`apply_patch`.

The accepted grammar is:

```text
*** Begin Patch
*** Add File: <path>
+<line>
...
*** Update File: <path>
*** Move to: <new-path>
@@
 <context>
-<removed>
+<added>
*** Delete File: <path>
*** End Patch
```

Patch input normalizes CRLF/CR to LF for parsing. Added files are written with
LF. Updates preserve an existing file's CRLF line endings when detected. Patch
lines are line-oriented and write a final newline; this version does not support
an EOF-no-newline marker.

The tool preflights all files before writing: parse errors, duplicate targets,
path/symlink escapes, missing update/delete sources, existing add/move targets,
directory targets, binary update sources, and context mismatches fail before any
write starts. Commit-time failures can still be partial; those return
`partial=true` with per-file statuses.

## Consequences

- Agents can make compact coordinated edits using a stable grammar.
- `file_edit` remains useful for simple anchored replacements.
- Rollback is not implemented; callers must inspect `partial` on failed
  results.
- No formatter, LSP, watcher, git, or telemetry side effects are part of the
  leaf tool.
