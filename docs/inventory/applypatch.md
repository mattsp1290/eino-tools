# Applypatch inventory

Added: 2026-06-26 for `eino-agent` coding-agent tool parity.

## Files

- `doc.go`: package contract, supported patch grammar, EOF/line-ending behavior,
  and concurrency contract.
- `applypatch.go`: parser, schema, preflight, workspace/path containment,
  context-match application, atomic writes, result envelope, and retry policy.
- `applypatch_test.go`: add/update/delete/move behavior, preflight failures,
  all-or-nothing preflight, symlink/path escapes, duplicate-key, schema, and
  RawJSON coverage.

## Public Surface

- Tool name: `Name = "apply_patch"`.
- Constants:
  - `MaxPatchBytes = 1024 * 1024`
- `Schema() json.RawMessage` returns a fresh copy.
- `New(workspacePath string)` rejects empty, non-absolute, and unresolvable
  workspace paths. It resolves the workspace root with `filepath.EvalSymlinks`.
- `Tool.Run(ctx, Args) Result`
- `Tool.Info(ctx) (*schema.ToolInfo, error)`
- `Tool.InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)`

## Schema Contract

The schema is an object with `additionalProperties: false`.

Required:

- `patch_text`

Properties:

- `patch_text`: string, `minLength: 1`, capped at 1 MiB by runtime validation.

## Patch Grammar

Supported operations:

- `*** Add File: <path>` with one or more `+` lines.
- `*** Update File: <path>` with one or more `@@` hunks, or with
  `*** Move to: <new-path>` for move/update.
- `*** Delete File: <path>`.

Hunk lines are line-oriented:

- space-prefixed lines are context.
- `-` lines are removed.
- `+` lines are added.

CRLF and CR patch input are normalized to LF for parsing. Added files use LF.
Updates preserve existing CRLF line endings when the source file uses CRLF.
Patch text is line-oriented and writes a final newline; no EOF-no-newline marker
is supported in this version.

## Preflight And Commit Semantics

Before any write starts, `apply_patch`:

- parses the whole patch;
- rejects duplicate target paths and parent/child planned target conflicts;
- rejects path escapes and symlink escapes;
- rejects missing update/delete sources;
- rejects existing add/move targets;
- rejects directory update/delete targets;
- rejects symlink update/delete sources, even when they point inside the
  workspace, so commit never removes a different path than the requested target;
- rejects binary or non-UTF-8 update sources;
- verifies every update hunk context matches exactly once after prior hunks for
  that file.

Writes are then applied sequentially with atomic temp-file replacement for add
and update. Because rollback is not implemented, commit-time failures return
`outcome=failed` with `partial=true` if any earlier file operation or move write
already succeeded. Preflight failures return `partial=false`.

No formatter, LSP, watcher, git, or telemetry side effects run inside this
package.

## Result Envelope

`Result` fields:

- `Outcome`
- `Files []FileResult`
- `Partial`
- `DiffTruncated`
- `Error`
- `RawJSON`

`FileResult` fields:

- `Operation`
- `Path`
- `NewPath`
- `Status`
- `Additions`
- `Deletions`

Error category strings:

- `validation`
- `path_escape`
- `not_found`
- `is_directory`
- `too_large`
- `unsupported`
- `conflict`
- `binary`
- `io`
- `canceled`
- `unknown`

Retryable result categories:

- `io`
- `unknown`

## Concurrency

`apply_patch` participates in the repository-level filesystem serialization
contract: callers must serialize `fileops`, `glob`, `search`, and `apply_patch`
calls per workspace root. Independent workspace roots may run concurrently.
