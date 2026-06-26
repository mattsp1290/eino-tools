# Fileops extraction inventory

Source inspected: `/home/infra-admin/git/local-symphony/internal/worker/tools/fileops`
on 2026-05-25.

## Files

- `doc.go`: package contract for workspace-rooted tools and observability boundary.
- `fileops.go`: shared constants, result envelope, error taxonomy, path helpers,
  duplicate-key guard, workspace validation, Eino `ToolInfo` helper.
- `read.go`: `file_read`, including legacy prefix reads and optional
  line-windowed reads.
- `write.go`: `file_write`.
- `edit.go`: `file_edit`.
- `list.go`: `file_list`.
- `fileops_test.go`: shared and per-tool behavior tests.

## Public API To Preserve

- Tool name constants:
  - `NameRead = "file_read"`
  - `NameWrite = "file_write"`
  - `NameEdit = "file_edit"`
  - `NameList = "file_list"`
- Tunables:
  - `MaxOutputBytes = 256 * 1024`
  - `DefaultReadWindowLines = 2000`
  - `MaxReadWindowLines = 5000`
  - `MaxReadLineBytes = 16 * 1024`
  - `MaxListEntries = 5000`
- Constructors:
  - `NewReadTool(workspacePath string) (*ReadTool, error)`
  - `NewWriteTool(workspacePath string) (*WriteTool, error)`
  - `NewEditTool(workspacePath string) (*EditTool, error)`
  - `NewListTool(workspacePath string) (*ListTool, error)`
- Schema helpers returning fresh `json.RawMessage` copies:
  - `ReadSchema()`
  - `WriteSchema()`
  - `EditSchema()`
  - `ListSchema()`
- Each tool exposes:
  - `Run(ctx, args) <Tool>Result`
  - `Info(ctx) (*schema.ToolInfo, error)`
  - `InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)`

## Schemas

All schema literals are object schemas with `additionalProperties: false`.

- `file_read`: requires `path`; optional `offset` and `limit` request a
  line-windowed read. Omitted offset/limit preserve the legacy prefix response.
- `file_write`: requires `path` and `content`; optional `create_dirs`.
- `file_edit`: requires `path`, `anchor`, and `replacement`.
- `file_list`: optional `path` and `recursive`; empty or omitted path lists `"."`.

Extraction must preserve fresh-copy schema behavior, valid JSON Schema parsing
through the existing validator, unknown-field rejection, and required-field
validation.

## Result Envelopes

All results embed `BaseResult`, flattening `outcome` and optional `error` into
the model-facing JSON object. Replace
`dispatcher.ToolOutcome` with `result.Outcome` during extraction.

- `BaseResult`: `Outcome`, `Error`, `IsRetryable()`.
- `ResultError`: `Category`, `Message`.
- `ReadResult`: `Path`, `Content`, `ContentBytes`, `Truncated`,
  `NumberedContent`, `LineStart`, `LineEnd`, `TotalLines`, `NextOffset`,
  `LineTruncated`, and `TruncationReason`.
- `WriteResult`: `Path`, `BytesWritten`, `Created`.
- `EditResult`: `Path`, `BytesWritten`, `AnchorOccurrences`.
- `ListEntry`: `Path`, `IsDir`.
- `ListResult`: `Path`, `Entries`, `Truncated`.

Error category strings to preserve:

- `validation`
- `path_escape`
- `not_found`
- `is_directory`
- `not_directory`
- `anchor_not_found`
- `anchor_ambiguous`
- `too_large`
- `binary`
- `io`
- `canceled`
- `unknown`

Retry policy is centralized in `BaseResult.IsRetryable`: only failed results
with category `io` or `unknown` are retryable.

## Workspace And Path Helpers

The extraction must move these helpers and their tests as a unit:

- `validateWorkspacePath`: workspace path must be non-empty and absolute.
- `canonicalizeWorkspace`: resolves symlinks in the workspace root when possible.
- `validateRelPath`: rejects empty, absolute, and NUL-containing paths.
- `resolveExisting`: resolves symlinks for existing read/edit/list targets and
  checks workspace containment.
- `resolveWritable`: checks syntactic containment, handles `create_dirs`, checks
  parent/ancestor symlinks, and refuses existing leaf symlinks escaping the
  workspace.
- `firstExistingAncestor`
- `syntacticDescendant`
- `isDescendant`
- `rejectDuplicateTopLevelKeys`
- `failed`, `failedFromPathErr`, `contextErrCategory`
- `atomicWrite` in `write.go`

The current TOCTOU caveat is part of the package contract: safety relies on the
agent loop serializing filesystem tools per workspace root. This includes
`fileops`, `glob`, `search`, and `apply_patch`. Concurrent tool execution
against independent workspace roots is allowed; concurrent calls against the
same workspace would require openat-style containment.

## Line-Windowed Reads

`file_read` remains backward-compatible for calls containing only `path`: it
returns the leading UTF-8 content prefix capped at `MaxOutputBytes` with the
legacy `content`, `content_bytes`, and `truncated` fields.

Supplying `offset` and/or `limit` switches to line-window mode:

- `offset` is 1-based and defaults to 1.
- `limit` defaults to 2000 and caps at 5000.
- `content` is the raw selected text without line numbers.
- `numbered_content` prefixes each selected line with `N: `.
- `line_start`, `line_end`, `total_lines`, `truncated`, and `next_offset`
  describe the window.
- `line_truncated` reports long-line truncation; `truncation_reason` is one of
  `prefix`, `lines`, `bytes`, or `line`.
- binary or non-UTF-8 files fail with `error.category="binary"`.

## Tests To Lift

`fileops_test.go` currently covers:

- Read success, truncation, missing paths, directories, path escape, symlink
  behavior, nil tool, context cancellation, and invokable JSON round trips.
- Write success, overwrite/create detection, `create_dirs`, parent missing,
  oversize content, path escape, symlink escape, atomic-write temp cleanup, nil
  tool, and context cancellation.
- Edit success, anchor not found, ambiguous anchors, empty anchor, oversize
  post-edit file, directory target, path escape, nil tool, and cancellation.
- List root/default path, non-recursive and recursive listing, sorted entries,
  truncation, file-vs-directory errors, path escape, nil tool, and cancellation.
- Schema validity, fresh schema slices, unknown-property rejection, required
  field rejection, stable names, helper behavior, retryability, and macOS
  symlinked temp-root canonicalization.

## Coupling To Replace

- Replace all imports of `github.com/mattsp1290/local-symphony/internal/dispatcher`
  with `github.com/mattsp1290/eino-tools/result`.
- Replace all `dispatcher.ToolOutcome*` constants with `result.Outcome*`.
- Keep Eino imports against `github.com/cloudwego/eino/components/tool` and
  `github.com/cloudwego/eino/schema`.
- Keep `github.com/eino-contrib/jsonschema` for the `buildToolInfo` helper
  unless a later implementation finds a local helper already covering this.
- Do not import local-symphony dispatcher, telemetry, auth, or `eino-ext`.
