# Changelog

This project uses a hand-curated changelog.

## Unreleased

### Added

- `glob` tool for doublestar path discovery under a workspace, including hidden
  paths by default while skipping VCS internals.
- `apply_patch` tool for multi-file add/update/delete/move patches with
  preflight, context-match verification, duplicate-target rejection, and
  per-file result summaries.
- Line-windowed `file_read` via optional `offset` and `limit`, including raw and
  numbered content, total line metadata, byte/line/long-line truncation signals,
  and binary-file rejection.

### Changed

- `search` now supports ripgrep glob filters, literal mode, ignore-case mode,
  context lines, configurable match limits up to 1000, and partial-result
  metadata while preserving existing calls.

### Migration Notes

- `eino-agent` must serialize workspace filesystem tools per workspace root:
  `fileops`, `glob`, `search`, and `apply_patch`. Independent workspace roots
  may run concurrently.
- `web_search` and LSP/diagnostics schemas remain runtime responsibilities for
  `eino-agent`, not reusable leaf tools in `eino-tools`.

## v0.1.0 - Pending

### Added

- Initial standalone module scaffolding.
- Shared `result.Outcome` enum.
- Workspace-rooted `fileops` tools:
  - `file_read`
  - `file_write`
  - `file_edit`
  - `file_list`
- Ripgrep-backed `search` tool with workspace path containment, match/byte
  caps, per-line truncation, and RawJSON result compatibility.
- `shell` execution tool preserving `sh -lc <cmd>`, timeout behavior,
  process-group cancellation, output caps, configurable env/shell binary/output
  cap, and RawJSON result compatibility.
- Close-only `tracker.CloseWriter` interface.
- `trackerwrite` tool. v0.1.0 executes only `op=close`; `comment`,
  `transition`, and `link_pr` remain parsed for schema compatibility but return
  `unsupported_op`.
- `tracker/beads` adapter over `github.com/mattsp1290/beads-go/beads`.
- CI checks for tests, lint, race tests, module tidiness, and dependency
  hygiene.
- Tagged integration coverage for `tracker/beads` against real `bd`.

### Changed

- Go baseline is 1.26 because `beads-go v0.1.0` declares `go 1.26`.

### Migration Notes

- `dispatcher.ToolOutcome` from `local-symphony` is replaced by
  `result.Outcome`. Consumers that still expose a dispatcher outcome should use
  a thin type alias or conversion shim during adoption.
- `trackerwrite` intentionally exposes only `tracker.CloseWriter` in v0.1.0.
  Consumers needing comments, transitions, or PR links must keep those
  operations in their own tracker layer until a later API promotion.
- `search` requires `rg` on `PATH`.
- `shell` executes model-provided commands by design. Workspace containment,
  network policy, secrets, and sandbox enforcement remain caller concerns.
- `tracker/beads` requires `bd` through `beads-go`'s runtime behavior unless
  consumers provide their own compatible `Close` client.

### Policy

- Pre-v1.0 releases may include breaking changes in minor versions.
- Every breaking change must include a migration note in this changelog before
  release.
