# Search extraction inventory

Source inspected: `/home/infra-admin/git/local-symphony/internal/worker/tools/search`
on 2026-05-25. Coding-agent parity extensions were added on 2026-06-26.

## Files

- `doc.go`: package contract, ripgrep backend rationale, path containment,
  exit-code policy, timeout behavior, streaming caps, UTF-8 behavior, and
  observability boundary.
- `search.go`: tool implementation, schema, path resolver, ripgrep execution,
  NDJSON parser, result envelope, truncation logic, duplicate-key guard, and
  retry policy.
- `export_test.go`: test-only seam for rebinding the ripgrep binary.
- `search_test.go`: constructor, path containment, execution, truncation,
  parsing, schema, retry, and concurrency tests.

## Public Surface To Preserve

- Tool name: `Name = "search"`.
- Constants:
  - `DefaultTimeoutSeconds = 60`
  - `MaxTimeoutSeconds = 600`
  - `MaxMatches = 200`
  - `MaxSearchLimit = 1000`
  - `MaxContextLines = 20`
  - `MaxLineBytes = 4 * 1024`
  - `MaxResultBytes = 256 * 1024`
  - `MaxStderrBytes = 4 * 1024`
- `Schema() json.RawMessage` returns a fresh copy.
- `New(workspacePath string)` rejects empty, non-absolute, and unresolvable
  workspace paths. It resolves the workspace root with `filepath.EvalSymlinks`.
- `Tool.Run(ctx, Args) Result` returns structured results for runtime outcomes.
- `Tool.Info(ctx)` returns Eino `*schema.ToolInfo`.
- `Tool.InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)`
  rejects empty, malformed, and duplicate top-level JSON before execution.

## Schema Contract

The schema is an object with `additionalProperties: false`.

Required:

- `pattern`

Properties:

- `pattern`: string, `minLength: 1`.
- `path`: optional string. Empty or omitted searches the workspace root.
- `timeout_seconds`: integer, `minimum: 0`, `maximum: 600`.
- `glob`: optional string or string array. Forwarded as repeated `rg -g`
  include filters.
- `literal`: optional boolean. When true, uses `rg -F`.
- `ignore_case`: optional boolean. When true, uses `rg -i`.
- `context`: optional integer, `minimum: 0`, `maximum: 20`. Uses `rg -C`.
- `limit`: optional integer, `minimum: 1`, `maximum: 1000`. Default remains
  `MaxMatches` (200) for backward compatibility.

`timeout_seconds` omitted or zero uses the 60 second default. The runtime keeps
defense-in-depth checks for negative and above-cap values.

## Ripgrep Invocation

The tool runs ripgrep as:

```text
rg --json [flags] -e <pattern> -- <path>
```

Execution details:

- `cmd.Dir` is the resolved workspace root.
- `<path>` is workspace-relative and defaults to `"."`.
- `literal`, `ignore_case`, `context`, and `glob` map to `-F`, `-i`, `-C`, and
  repeated `-g` flags respectively.
- `-e` prevents patterns beginning with `-` from being interpreted as flags.
- `--` prevents paths beginning with `-` from being interpreted as flags.
- `stdin` is an empty reader.
- `cmd.WaitDelay` is five seconds after cancellation.
- The runtime dependency is `rg` on `PATH`; missing binary maps to
  `exec_failed`.

ADR 0005 says v0.1 must keep ripgrep rather than reimplementing search in Go.

## Path Resolver Semantics

Path resolution happens before spawning ripgrep.

- Empty `Args.Path` becomes `"."`; then `filepath.Clean` normalizes the value.
- Absolute paths and NUL-containing paths fail validation.
- The candidate path is `workspacePath + rel`.
- `filepath.EvalSymlinks` resolves existing path components.
- Missing paths return `not_found`.
- Paths resolving outside the workspace return `path_escape`.
- The workspace root itself is allowed.
- Both files and directories are valid search roots because ripgrep accepts both.

Model-facing match paths are workspace-relative. A leading `"./"` from ripgrep
is stripped so returned paths compose with `file_read` and `file_edit`.

## Result Envelope

`Result` fields:

- `Outcome`: replace `dispatcher.ToolOutcome` with `result.Outcome`.
- `Matches []Match`
- `MatchCount`
- `Truncated`
- `TruncationReason`
- `DurationMS`
- `TimedOut`
- `Partial`
- `Error`

`Match` fields:

- `Path`
- `LineNumber`
- `Line`
- `LineTruncated`
- `Submatches []Submatch`
- `Before []ContextLine`
- `After []ContextLine`

`ContextLine` fields:

- `LineNumber`
- `Line`
- `LineTruncated`

`Submatch` fields:

- `Text`
- `Start`
- `End`

`ResultError` fields:

- `Category`
- `Message`

Error category strings to preserve:

- `validation`
- `path_escape`
- `not_found`
- `invalid_pattern`
- `timeout`
- `canceled`
- `exec_failed`
- `unknown`

Retryable result categories:

- `timeout`
- `unknown`

## Exit And Timeout Policy

- Ripgrep exit 0 -> `outcome=succeeded`.
- Ripgrep exit 1 -> `outcome=succeeded` with an empty `matches` slice and
  `match_count=0`.
- Ripgrep exit 2 -> `outcome=failed`, `error.category=invalid_pattern` when no
  useful matches were parsed.
- Ripgrep exit 2 or unexpected non-zero with parsed matches ->
  `outcome=succeeded`, `partial=true`, and `error.category=exec_failed`.
- Unexpected non-zero with no parsed matches -> `outcome=failed`,
  `error.category=exec_failed`.
- Cap-driven cancellation after enough useful matches were parsed is
  reclassified as success with `truncated=true`, even if ripgrep exits non-zero
  due to cancellation.
- Parent context cancellation/deadline maps to `canceled` unless cap-driven
  truncation already locked in a useful success result.
- Tool-created per-call timeout maps to `timeout` with `timed_out=true`.

## Streaming And Output Caps

Ripgrep stdout is parsed as NDJSON through `bufio.Scanner` with an 8 MiB
per-line scanner cap.

`match` and `context` events are consumed. Other events and malformed match
lines are skipped defensively. A scanner overflow returns `unknown`.

Parsing stops when:

- another match would exceed the effective `limit`, producing `truncated=true`
  and `truncation_reason="matches"`.
- cumulative retained match and context line bytes reach `MaxResultBytes`,
  producing `truncated=true` and `truncation_reason="bytes"`.

Each matched line is capped at `MaxLineBytes`; trimmed lines set
`line_truncated=true`. Submatches whose offsets no longer fit inside a
truncated line are dropped. Literal mode omits regex submatches from the
model-facing result.

For non-UTF-8 files, ripgrep emits base64 `bytes`; the wrapper decodes those and
uses `strings.ToValidUTF8(..., "�")` for printable model-facing text.

Stderr is captured through a capped 4 KiB buffer. Displayed ripgrep error
messages are trimmed to 1 KiB with UTF-8 boundary handling.

## Coupling To Replace

- Replace `local-symphony/internal/dispatcher.ToolOutcome*` with
  `eino-tools/result.Outcome*`.
- Keep Eino imports against `github.com/cloudwego/eino/components/tool` and
  `github.com/cloudwego/eino/schema`.
- Keep `github.com/eino-contrib/jsonschema` for schema-to-ToolInfo conversion
  unless a shared helper replaces it.
- Keep `os/exec` for this package per ADR 0005.
- Do not import local-symphony dispatcher, telemetry, auth, core, worker, or
  fileops packages.

## Tests To Lift

The implementation work should lift or recreate tests for:

- Constructor validation, absolute path success, and unresolvable workspace
  rejection.
- Nil receiver, empty/NUL pattern, timeout bounds, and already-canceled context.
- Path escape by `..`, absolute path, NUL path, and symlink escape.
- Missing path -> `not_found`.
- No matches -> success with empty `matches` and `match_count=0`.
- Basic match parsing, path scoping, file-as-root search, default root path, and
  patterns beginning with `-`.
- Invalid regex -> `invalid_pattern`.
- Missing rg binary -> `exec_failed`.
- Match cap, byte cap, per-line truncation, stderr cap, and scanner overflow.
- Parent cancellation and explicit timeout behavior.
- Invokable JSON round trip, empty/malformed JSON, and duplicate top-level key
  rejection.
- Schema shape, fresh schema slices, timeout bound declarations, and stable
  `Name`.
- Retry policy.
- Base64/non-UTF-8 line decoding.
- Parser behavior for non-match events, malformed JSON, newline trimming,
  out-of-range submatch dropping, golden fixture parsing, result JSON shape,
  duration population, and concurrency safety under `-race`.
