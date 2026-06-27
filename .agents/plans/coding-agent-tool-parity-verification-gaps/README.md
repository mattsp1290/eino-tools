# Coding-Agent Tool Parity Verification Gaps

Source request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-27-coding-agent-tool-parity-verification-gaps.md`

Project: `eino-tools`

Change type: defect closure / verification hardening

## Goal

Close the remaining verification gaps for the coding-agent tool parity work that landed at commit `19e3aa4f7bbcea842ced9448896f64ace1304ae2`, so `github.com/mattsp1290/eino-agent` has a passing verification gate and a response artifact with an exact commit or tag to pin.

This plan intentionally does not reimplement the completed parity work. It only addresses:

1. `golangci-lint run` failures in `applypatch/applypatch.go` and `fileops/read.go`.
2. `search` classification of ripgrep exit-code-2 failures with zero matches.
3. Tests and docs for that `search` classification edge case.
4. A response artifact under `/Users/punk1290/.agents/projects/eino-tools/` for the original parity request.

## Affected Areas

- `applypatch/applypatch.go`: narrow gosec `G703` fixes/suppressions around `atomicWrite` temp-path cleanup and rename.
- `fileops/read.go`: staticcheck `QF1012` simplification in line-windowed numbered content construction.
- `search/search.go`: ripgrep exit-code classification near the exit-code switch, plus any helper used to classify stderr.
- `search/search_test.go`: regression tests for invalid regex versus inaccessible path / permission failures, including the partial-with-matches path if feasible.
- `docs/inventory/search.md`: align exit-code policy docs with the new invalid-pattern versus exec-failed split.
- `/Users/punk1290/.agents/projects/eino-tools/responses/`: create the missing response artifact for `2026-06-26-coding-agent-tool-parity-for-eino-agent.md`.

## Current Signals

Recent history shows the parity work is already present:

- `977f4a8 Add coding-agent tool parity`
- `2dcefbf Fix coding-agent parity review issues`
- `b77f54d Fix second review edge cases`
- `19e3aa4 Merge branch 'feat/harness-parity'...`

The request reports:

```text
go test ./...      # passes
go vet ./...       # passes
golangci-lint run  # fails
```

The lint failures are:

```text
applypatch/applypatch.go:751:16: G703: Path traversal via taint analysis (gosec)
applypatch/applypatch.go:754:21: G703: Path traversal via taint analysis (gosec)
applypatch/applypatch.go:755:16: G703: Path traversal via taint analysis (gosec)
fileops/read.go:311:6: QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) (staticcheck)
```

The behavioral gap is that `search/search.go` maps all ripgrep exit code `2` with no parsed matches to `invalid_pattern`. That remains correct for regex parse errors, but inaccessible-path and permission errors must be `exec_failed`.

## Design Decisions

- Prefer a small stderr classifier in `search` over broad command-behavior changes. Keep the existing partial-success behavior when ripgrep returns any parsed matches.
- Keep invalid regex classification stable for callers: invalid regex remains `outcome=failed`, `error.category="invalid_pattern"`.
- Treat ripgrep access/execution diagnostics as `exec_failed`: permission denied, operation not permitted, I/O errors, failed directory access, and similar `rg:` runtime diagnostics.
- Use narrow `//nolint:gosec` comments only where the path has already passed existing workspace containment and target preflight. Do not blanket-disable gosec for the file or package.
- Create the response artifact after verification, because it must include the final commit or tag. The implementing agent should use `git rev-parse HEAD` after committing fixes and record that value.

## Constraints

- Preserve public schemas, JSON fields, exported constants, tool names, and backward-compatible behavior except for the requested error category correction.
- Keep path containment and symlink-escape protections unchanged.
- Do not invent LSP, web-search, concurrency, telemetry, or permission-prompting behavior; ADR 0008 already documents ownership boundaries.
- Tests that rely on permissions should skip on platforms where the setup is not meaningful, especially Windows or root-like environments where chmod denial does not behave normally.

