# Implementation Plan

Use this file as the execution guide for the coding agent. Track actual work with `bd`; do not use this markdown file as a live task tracker.

## 1. Start and Baseline

Create or claim a bead for the implementation work, then run the baseline commands from the repo root:

```bash
go test ./...
go vet ./...
golangci-lint run
```

Record any deviations from the request before editing. If new failures appear outside the listed files, decide whether they are caused by this branch; unrelated drift should become a follow-up bead rather than being folded into this change.

## 2. Fix Lint Failures

### `fileops/read.go`

At the line-windowed read path, replace:

```go
numbered.WriteString(fmt.Sprintf("%d: %s", totalLines, line))
```

with the staticcheck-preferred equivalent:

```go
fmt.Fprintf(&numbered, "%d: %s", totalLines, line)
```

Make sure the `fmt` import remains needed elsewhere before changing imports.

### `applypatch/applypatch.go`

Inspect `atomicWrite` and the preflight path that calls it. The reported `G703` warnings are on temp cleanup and rename operations:

```go
_ = os.Remove(tmpPath)
if err := os.Rename(tmpPath, path); err != nil { ... }
```

Preferred fix:

- Confirm `path` reaches `atomicWrite` only from resolved workspace-contained paths produced by the apply-patch preflight.
- Add narrow `//nolint:gosec` comments on the exact `os.Remove` and `os.Rename` calls that gosec reports.
- The comments should explain both sides of the safety argument: `tmpPath` comes from `os.CreateTemp` inside the already-resolved target directory, and `path` has already passed workspace containment / symlink preflight.

Avoid changing the apply-patch commit semantics unless a lint-compliant helper is clearly simpler and fully covered by existing tests.

## 3. Correct `search` Exit-Code-2 Classification

In `search/search.go`, keep the current exit-code switch shape but replace the zero-match `exitCode == 2` branch with stderr-based classification.

Current behavior:

```go
case exitCode == 2:
	if len(matches) > 0 {
		res.Outcome = result.OutcomeSucceeded
		res.Partial = true
		res.Error = &ResultError{Category: ErrCategoryExecFailed, Message: rgErrorMessage(stderrBuf, waitErr)}
		return res
	}
	res.Outcome = result.OutcomeFailed
	res.Error = &ResultError{Category: ErrCategoryInvalidPattern, Message: rgErrorMessage(stderrBuf, waitErr)}
	return res
```

Target behavior:

- `len(matches) > 0`: unchanged partial success with `ErrCategoryExecFailed`.
- `len(matches) == 0` and stderr indicates an invalid regex/pattern: failed with `ErrCategoryInvalidPattern`.
- `len(matches) == 0` and stderr indicates permission, inaccessible path, filesystem, or other ripgrep runtime failure: failed with `ErrCategoryExecFailed`.

Implementation guidance:

- Add an unexported helper near `rgErrorMessage` or near the exit switch, for example `rgExit2ErrorCategory(stderr string) string`.
- Classify only ripgrep diagnostics that are anchored to regex/pattern parse output as invalid pattern. Prefer exact structures observed from `rg`, such as a line containing `regex parse error:`, rather than broad substrings like `unclosed` or `look-around` that could appear in filenames or runtime diagnostics.
- Default unknown exit-code-2 diagnostics to `ErrCategoryExecFailed`, not `ErrCategoryInvalidPattern`. This is safer for the request because only known regex parse failures should be pattern errors.
- Keep the error message from `rgErrorMessage(stderrBuf, waitErr)` so callers still get the raw ripgrep context.

## 4. Add Regression Tests

Add focused tests in `search/search_test.go`.

### Invalid Regex

Add or update a test that runs a syntactically invalid regex, such as:

```go
res := tool.Run(context.Background(), Args{Pattern: "["})
```

Assert:

- `res.Outcome == result.OutcomeFailed`
- `res.Error != nil`
- `res.Error.Category == ErrCategoryInvalidPattern`
- `res.MatchCount == 0`

### Inaccessible Path With Zero Matches

Create a directory under the temp workspace that ripgrep cannot read, then run a search scoped to that path.

Test shape:

- Skip on Windows.
- Create `blocked/` with mode `0o000` after writing any setup files.
- Use `defer os.Chmod(blocked, 0o700)` so temp cleanup can remove it.
- If the process can still read the directory, skip rather than failing; root-like environments can bypass permissions.
- Run `tool.Run(context.Background(), Args{Pattern: "needle", Path: "blocked"})`.
- Assert failed outcome with `ErrCategoryExecFailed`, zero matches, and an error message containing useful ripgrep stderr.

If ripgrep returns exit 1 rather than 2 in the local environment, adjust setup so ripgrep actually attempts to descend into a denied subdirectory. For example, search the workspace root with one readable file absent of matches and one unreadable child directory.

### Partial With Matches Plus Inaccessible Path

Add deterministic coverage for this required behavior:

- Prefer a fake `rg` test binary/script injected into `Tool.rgBinary` from inside the `search` package tests.
- Have the fake emit one valid JSON match to stdout, write a permission-style diagnostic to stderr, and exit with status `2`.
- Assert `OutcomeSucceeded`, `Partial == true`, `Error.Category == ErrCategoryExecFailed`, and at least one returned match.

Keep chmod-based permission tests as optional integration coverage only. The partial-with-matches path must not depend on filesystem permission behavior that varies by OS, container user, or root-like execution.

## 5. Update Documentation

Update `docs/inventory/search.md` under "Exit And Timeout Policy".

The docs should say:

- Ripgrep exit 2 with no parsed matches is `invalid_pattern` only when stderr is classified as a regex/pattern parse failure.
- Ripgrep exit 2 with no parsed matches is `exec_failed` for inaccessible paths, permission failures, or other runtime execution errors.
- Ripgrep exit 2 or unexpected non-zero with parsed matches remains successful but partial with `error.category=exec_failed`.

Do not alter unrelated ownership boundaries in ADR 0008 unless the implementation materially changes them, which this plan does not expect.

## 6. Create the Response Artifact

After implementation, tests, the final repo commit, and any required `git pull --rebase` are complete, create:

```text
/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md
```

Create the `responses/` directory if needed. This artifact is outside the `eino-tools` git repo; the pin it records should be the actual commit that will be pushed to `github.com/mattsp1290/eino-tools`, not a speculative pre-rebase SHA.

The artifact must include:

- Original request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-26-coding-agent-tool-parity-for-eino-agent.md`
- Verification-gap request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-27-coding-agent-tool-parity-verification-gaps.md`
- Completion status for the original parity request.
- Exact commit or tag for `eino-agent` to pin, from `git rev-parse HEAD` after the final commit and after any rebase.
- Verification commands and results:
  - `go test ./...`
  - `go vet ./...`
  - `golangci-lint run`
- Search implementation choice and the exit-code-2 classification fix.
- Workspace filesystem serialization contract and granularity required by `eino-agent`.
- `web_search` ownership decision.
- LSP/diagnostics ownership decision.
- Remaining intentional deferrals, if any. If none, state `None`.

Do not fill in a speculative commit hash before the final implementation commit exists and the branch has been rebased onto the push target.

## 7. Final Verification and Closeout

Run:

```bash
go test ./...
go vet ./...
golangci-lint run
git status --short
```

Then follow the repo close protocol. Because the response artifact is outside this repo and must record the pushed commit, commit and rebase before writing the response artifact:

```bash
git add <changed-files>
git commit -m "Close coding-agent parity verification gaps"
git pull --rebase
PIN=$(git rev-parse HEAD)
mkdir -p /Users/punk1290/.agents/projects/eino-tools/responses
# Write /Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md with PIN.
bd close <implementation-bead-id> --reason="Verification gaps closed"
bd dolt push
git push
git status
```

`git status` must show the branch up to date with origin before the work is considered complete.
