# Glob inventory

Added: 2026-06-26 for `eino-agent` coding-agent tool parity.

## Files

- `doc.go`: package contract, doublestar dialect, ignore-file note, and
  concurrency contract.
- `glob.go`: tool implementation, schema, workspace/path containment, walk
  logic, VCS skipping, duplicate-key guard, result envelope, and retry policy.
- `glob_test.go`: behavior, containment, schema, duplicate-key, and RawJSON
  coverage.

## Public Surface

- Tool name: `Name = "glob"`.
- Constants:
  - `DefaultLimit = 1000`
  - `MaxLimit = 5000`
- `Schema() json.RawMessage` returns a fresh copy.
- `New(workspacePath string)` rejects empty, non-absolute, and unresolvable
  workspace paths. It resolves the workspace root with `filepath.EvalSymlinks`.
- `Tool.Run(ctx, Args) Result`
- `Tool.Info(ctx) (*schema.ToolInfo, error)`
- `Tool.InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)`

## Schema Contract

The schema is an object with `additionalProperties: false`.

Required:

- `pattern`

Properties:

- `pattern`: string, `minLength: 1`. Uses
  `github.com/bmatcuk/doublestar/v4` semantics (`*`, `?`, character classes,
  and `**` recursive directory matching).
- `path`: optional workspace-relative search directory. Empty or omitted means
  `"."`.
- `limit`: optional integer, `minimum: 1`, `maximum: 5000`. Runtime default is
  1000.

## Behavior

- Walks only under the configured workspace root.
- Rejects absolute paths, NUL bytes, parent traversal in patterns, path escapes,
  and symlink escapes.
- Includes hidden files and directories by default.
- Skips VCS internals `.git`, `.hg`, `.svn`, and `.jj`.
- Does not read or respect `.gitignore` in this version; that is documented as
  deferred because this is a pure-Go walker rather than an `rg --files` wrapper.
- Returns sorted workspace-relative paths with `/` separators.
- Directory entries use `is_dir=true`.

## Result Envelope

`Result` fields:

- `Outcome`
- `Paths []PathEntry`
- `Count`
- `Truncated`
- `Error`
- `RawJSON`

`PathEntry` fields:

- `Path`
- `IsDir`

Error category strings:

- `validation`
- `path_escape`
- `not_found`
- `not_directory`
- `io`
- `canceled`
- `unknown`

Retryable result categories:

- `io`
- `unknown`

## Concurrency

`glob` participates in the repository-level filesystem serialization contract:
callers must serialize `fileops`, `glob`, `search`, and `apply_patch` calls per
workspace root. Independent workspace roots may run concurrently.
