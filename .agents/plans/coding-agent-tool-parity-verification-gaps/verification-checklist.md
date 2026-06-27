# Verification Checklist

Use this as a handoff checklist after implementation. The actual source of truth for task state is Beads.

## Required Commands

Run from `/Users/punk1290/git/eino-tools`:

```bash
go test ./...
go vet ./...
golangci-lint run
```

All three must pass.

## Required Behavioral Assertions

`search` must distinguish ripgrep exit-code-2 causes:

- Invalid regex: `outcome=failed`, `error.category="invalid_pattern"`, zero matches.
- Inaccessible path or permission failure with zero matches: `outcome=failed`, `error.category="exec_failed"`, zero matches.
- Inaccessible path or permission failure with parsed matches: `outcome=succeeded`, `partial=true`, `error.category="exec_failed"`.

`apply_patch` and `file_read` behavior must remain otherwise unchanged.

## Response Artifact Requirements

The response artifact should live at:

```text
/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md
```

It should include:

- Status of the original parity request.
- Final commit or tag for `eino-agent` to pin.
- Verification command results with dates.
- Search implementation choice and the exit-code-2 classifier correction.
- Workspace filesystem serialization contract and required granularity for `eino-agent`.
- `web_search` ownership decision.
- LSP/diagnostics ownership decision.
- Intentional deferrals, or `None`.

Suggested shape:

```markdown
# Response: Coding-Agent Tool Parity for Eino Agent

Original request: ...
Verification-gap request: ...
Status: Complete
Pin: <commit-or-tag>

## Scope Decisions

- Search implementation: <ripgrep-backed search choice and exit-code-2 classification note>
- Workspace filesystem serialization: <whether eino-agent must serialize fileops/glob/search/apply_patch calls and at what granularity>
- Web search ownership: <eino-agent/eino-tools decision>
- LSP/diagnostics ownership: <eino-agent/eino-tools/later-package decision>

## Verification

- `go test ./...`: passed on <date>
- `go vet ./...`: passed on <date>
- `golangci-lint run`: passed on <date>

## Notes

- `search` now classifies ripgrep exit-code-2 inaccessible-path/runtime failures as `exec_failed` while preserving invalid regex as `invalid_pattern`.
- Intentional deferrals: None
```

## Closeout

Before ending the implementation session:

```bash
bd close <completed-beads>
git add <changed-files>
git commit -m "Close coding-agent parity verification gaps"
git pull --rebase
bd dolt push
git push
git status
```

The final `git status` must show the branch is up to date with origin.
