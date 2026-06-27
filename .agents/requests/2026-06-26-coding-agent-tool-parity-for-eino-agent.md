# Request: Coding-agent tool parity for `eino-agent`

- **Requested by:** `eino-agent` planning in `~/git/eino-agent`
- **Date:** 2026-06-26
- **Priority:** High - blocks `eino-agent` from providing a productive coding-agent harness without falling back to ad hoc shell commands for core navigation and edits.
- **Target repo:** `~/git/eino-tools` (`github.com/mattsp1290/eino-tools`)
- **Consumer:** `github.com/mattsp1290/eino-agent`

## Background

`eino-agent` is intended to run coding agents under a Go/Eino harness. Comparing the current `eino-tools` surface with `pi` and `opencode` shows that `eino-tools` has a solid base:

- `fileops`: `file_read`, `file_write`, `file_edit`, `file_list`
- `search`: ripgrep-backed content search
- `shell`
- `url_fetch`
- `user_interact`
- `tracker_write`

That is enough for a minimal agent, but not enough for a strong coding agent to work efficiently and safely in large repos. `pi` adds dedicated file discovery (`find`), line-windowed reads, richer grep controls, and remote-operation seams. `opencode` adds `glob`, `apply_patch`, richer `read`, richer `grep`, `webfetch`, `websearch`, `lsp`, `task`, `todowrite`, `skill`, and runtime permission/output-retention behavior.

Some of those belong in `eino-agent` rather than `eino-tools`: `task`/subagents, `todowrite`, `skill`, permissions, session state, and output retention are runtime/session responsibilities. The reusable leaf tools below should live in or be exposed from `eino-tools` so `eino-agent` does not duplicate them.

## Blocking Asks

These asks are the upstream work that should complete before `eino-agent` starts relying on `eino-tools` for coding-agent leaf tools.

### 1. Add a dedicated `glob` file-discovery tool

Add package `glob` and model-facing tool name `glob`. This should be a companion to `file_list`, not a replacement: `file_list` lists a known directory; `glob` discovers paths by pattern.

Proposed schema:

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "pattern": {
      "type": "string",
      "minLength": 1,
      "description": "Glob pattern to match files against, e.g. \"*.go\" or \"**/*_test.go\"."
    },
    "path": {
      "type": "string",
      "description": "Workspace-relative directory to search. Omit or use \".\" for the workspace root."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 5000,
      "description": "Maximum number of paths to return. Default 1000; hard cap 5000."
    }
  },
  "required": ["pattern"]
}
```

Required behavior:

- Use doublestar-style glob semantics (`*`, `?`, `**`) and document the exact dialect.
- Search only under the configured workspace root; reject absolute paths, `..` escapes, NUL bytes, and symlink escapes.
- Include hidden files and directories by default, except `.git/` and other VCS internals unless a later option explicitly includes them.
- Respect `.gitignore` if using `rg --files -g`; if using a pure-Go implementation, either respect `.gitignore` or explicitly document that ignore-file support is deferred.
- Return sorted workspace-relative paths using `/` separators; directory entries should have a trailing `/` or an `is_dir` boolean.
- Result envelope should include `outcome`, `paths`, `count`, `truncated`, optional `error`, and `RawJSON` preservation.

Rationale: today `eino-tools/search` is content-regex only. Agents need fast path discovery without shelling out to `find`, `fd`, `rg --files`, or `ls` loops.

### 2. Upgrade `file_read` with backward-compatible line-window support

Extend the existing `fileops.ReadArgs` and `file_read` schema in place rather than adding a new model-facing read tool. Existing calls with only `path` must keep returning the current raw `content` prefix shape so existing consumers do not break.

Add optional fields:

- `offset`: integer, 1-based starting line, default `1`, minimum `1`.
- `limit`: integer, number of lines to return, default `2000`, maximum `5000`.

Required behavior:

- When `offset` or `limit` is present, return a line-windowed result with line numbers and metadata while still keeping a machine-usable raw content field:
  - `content`: raw selected text without line numbers, suitable for `file_edit` anchors.
  - `numbered_content`: selected text with `N: ` prefixes.
  - `line_start`, `line_end`, `total_lines`, `truncated`, `next_offset`.
- Enforce both line and byte caps. Keep the existing `MaxOutputBytes` or introduce clearly documented `MaxReadWindowBytes`; do not let a single tool call dump an unbounded file.
- Long lines should be truncated independently with a marker and a `line_truncated` signal.
- If `offset` is past EOF, return `outcome=failed` with `error.category="validation"` and a message naming the total line count.
- Binary files should return `outcome=failed` with a clear `binary` or `unsupported` category, rather than dumping arbitrary bytes.
- Directory reads and image/PDF attachments are **out of scope for `eino-tools` in this request**. Keep `file_list` for directories. Media attachment handling belongs in `eino-agent`/AG-UI.

Rationale: the current `file_read` returns only a leading 256 KiB prefix. For large files this wastes context, hides line numbers, and forces shell fallbacks such as `sed -n`.

### 3. Add a multi-file `apply_patch` tool

Add package `patch` or `applypatch` with model-facing tool name `apply_patch`. Keep `file_edit` for simple anchored edits; `apply_patch` is for multi-file structured changes.

Use this patch grammar unless a response justifies a compatible alternative:

```text
*** Begin Patch
*** Add File: <path>
+<line>
...
*** Update File: <path>
*** Move to: <new-path>        # optional, only for update hunks
@@ or @@ <context>
 <space context line>
-<removed line>
+<added line>
...
*** Delete File: <path>
*** End Patch
```

Required behavior:

- One argument: `patch_text` string. Reject empty patches and patches over a documented size cap, suggested 1 MiB.
- Support add, update, delete, and move. If move is deferred, reject it with `unsupported` and say so in docs.
- Preflight every target before writing: parse all hunks, reject duplicate target paths, reject path escapes, reject missing update/delete files, reject existing add targets unless explicitly allowed by grammar.
- Apply update/delete only when context matches the current file content. Preserve existing file mode for updates. Create parent directories for adds and moves.
- Normalize CRLF/CR to LF for patch parsing, but preserve existing file line endings on update when possible. Ensure added text ends with the intended final newline; document EOF behavior.
- Prefer all-or-nothing by deriving all new file contents before writing. If true rollback is not implemented, no write should start until all preflight/context checks pass; any commit-time partial write must return explicit `partial=true` with per-file status.
- No formatter, LSP, watcher, git, or telemetry side effects inside `eino-tools`.
- Result envelope should include `outcome`, `files` with operation/path/status/additions/deletions, `partial`, `diff_truncated`, optional `error`, and `RawJSON`.

Rationale: coding agents frequently need coordinated edits across files. A patch tool is safer and more compact than repeated write/edit calls or shelling out.

### 4. Add richer content-search controls

Extend `search` compatibly if possible; otherwise add a model-facing `grep` companion and leave `search` stable. The final response should state which path was chosen.

Proposed additional fields:

- `glob`: optional string or string array, passed to ripgrep as include filters.
- `literal`: optional boolean, default false, maps to fixed-string search.
- `ignore_case`: optional boolean, default false.
- `context`: optional integer, default 0, maximum 20, same number of before/after context lines.
- `limit`: optional integer, default existing `MaxMatches` or 100, maximum 1000.

Required behavior:

- Preserve current `search` behavior for existing calls.
- Results should include file path, line number, matching line, submatches when regex mode is used, and optional `before`/`after` context arrays with line numbers.
- Long lines, total result bytes, and match count must be capped with explicit `truncated` and `truncation_reason` metadata.
- Ripgrep inaccessible-path or partial errors should surface as `exec_failed` or `partial` metadata rather than silently pretending the search was complete.

Rationale: current `search` can find content, but agents often need literal search, file-type filters, and local context without opening many files.

### 5. Concurrency and containment contract

`eino-agent` will eventually run concurrent sessions and may run independent tool calls in parallel. Current `eino-tools/fileops` docs note a TOCTOU caveat: safety relies on the agent loop running one tool call at a time.

Pick one of these and document it:

- Harden file mutation/read containment against symlink-swap races enough for concurrent calls, or
- Expose a package-level/workspace-level serialization primitive or explicit contract that `eino-agent` must honor around fileops, glob, search, and patch calls.

The response must tell `eino-agent` whether it must serialize workspace filesystem tools and at what granularity.

## Non-Blocking Decisions

These should not block the file/search/patch parity slice, but the response should record ownership.

### Web search

Prefer declaring `web_search` out of scope for `eino-tools` unless the repo wants a provider-neutral, credential-free abstraction. Provider credentials, rate limits, freshness policy, permissions, citations, and browsing safety are runtime connector concerns and likely belong in `eino-agent`.

At minimum, the response should say whether `eino-agent` should own the model-facing `web_search` schema.

### LSP / diagnostics

Prefer declaring LSP/diagnostics out of scope for `eino-tools` unless the repo wants a separate optional package with explicit server lifecycle. LSP server startup/configuration, indexing state, diagnostics timing, permissions, line/column conventions, and cancellation are runtime-heavy and likely belong in `eino-agent` or a later `eino-lsp` style package.

At minimum, the response should say whether `eino-agent` should own the model-facing LSP/diagnostics schema.

## Out of scope

- `task`/subagent orchestration. That belongs in `eino-agent`.
- `todowrite`/plan state. That belongs in `eino-agent` session state unless a later design extracts a generic stateful tool contract.
- `skill` loading. That belongs in `eino-agent` runtime/context handling.
- Permission prompting and approval persistence. `eino-tools` can expose metadata, but `eino-agent` owns policy and user interaction flow.
- Tool output retention/spooling. `eino-tools` should cap outputs and report truncation; `eino-agent` owns durable managed-output storage.
- Image/PDF AG-UI attachments for reads. That belongs in `eino-agent`/`eino-agui`.
- Datadog observability. That belongs in `eino-obs` and `eino-agent` instrumentation wrappers.
- AG-UI event emission. That belongs in `eino-agui`.
- Browser automation / Playwright. Useful for frontend agents, but not part of parity with the inspected `pi`/`opencode` coding-tool baseline.

## Acceptance

- `eino-tools` exposes `glob` and upgraded `file_read` behavior that let an agent explore a large repo without shell fallbacks for `rg --files`, `find`, `ls`, `sed`, or `head`.
- `file_read` remains backward-compatible for existing `{path}` calls and adds tested line-window behavior for `offset`/`limit`, including EOF, byte caps, line caps, long-line truncation, and binary-file failures.
- `eino-tools` exposes `apply_patch` with the documented grammar, containment checks, duplicate-target rejection, context-match verification, clear all-or-partial semantics, and per-file result summaries.
- Content search supports glob filtering, literal mode, ignore-case mode, context lines, match limits, and explicit truncation metadata.
- Web search and LSP ownership are decided explicitly without blocking the file/search/patch parity slice.
- The concurrency/containment response states whether `eino-agent` must serialize workspace filesystem tools, and at what granularity.
- New/changed tools keep the current `eino-tools` conventions: `Info(ctx)`, `InvokableRun(ctx, argsJSON, ...tool.Option)`, fresh schema helpers, duplicate top-level key rejection, stable result envelopes, `RawJSON` preservation, and no telemetry/logging dependencies.
- Tests cover per-tool happy paths, schema validation, duplicate-key rejection, path escape and symlink escape, truncation, binary handling where applicable, patch preflight failures, patch partial/commit-time failure behavior, and compatibility for existing `file_read`, `file_list`, `file_edit`, and `search` calls.
- Docs are updated: README package list/examples, inventory docs for new/changed tools, ADRs for patch grammar and concurrency/containment, and CHANGELOG entry with migration notes.
- Verification commands pass in `~/git/eino-tools`: `go test ./...`, `go vet ./...`, `golangci-lint run` if configured, and any package-specific integration tests documented by the implementation.
- A tag or commit is recorded in the response for `eino-agent` to pin.

## References

- Consumer plan: `~/git/eino-agent/docs/prompts/eino-agent-go-runtime-for-ag-ui-and-datadog.md`
- Current `eino-tools` README and inventory docs: `~/git/eino-tools/README.md`, `~/git/eino-tools/docs/inventory/*.md`
- `pi` tool reference: `https://github.com/earendil-works/pi`, especially `packages/coding-agent/src/core/tools/`
- `opencode` tool reference: `https://github.com/anomalyco/opencode`, especially `packages/opencode/src/tool/` and `packages/core/src/tool/`
