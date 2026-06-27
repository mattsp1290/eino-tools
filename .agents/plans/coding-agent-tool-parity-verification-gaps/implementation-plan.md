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
- Classify common ripgrep regex diagnostics as invalid pattern. Test against the exact stderr emitted by installed `rg` rather than depending on one phrase only. Useful indicators may include phrases such as `regex parse error`, `unclosed`, `look-around`, or ripgrep's pattern diagnostic format.
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

Add this if the platform setup is reliable:

- Create a readable file containing `needle`.
- Create a denied directory in the same searched root.
- Search from workspace root.
- Assert `OutcomeSucceeded`, `Partial == true`, `Error.Category == ErrCategoryExecFailed`, and at least one returned match.

If this is too environment-sensitive, keep the helper unit-testable by adding a table test for the stderr classifier and rely on the zero-match integration test for end-to-end coverage.

## 5. Update Documentation

Update `docs/inventory/search.md` under "Exit And Timeout Policy".

The docs should say:

- Ripgrep exit 2 with no parsed matches is `invalid_pattern` only when stderr is classified as a regex/pattern parse failure.
- Ripgrep exit 2 with no parsed matches is `exec_failed` for inaccessible paths, permission failures, or other runtime execution errors.
- Ripgrep exit 2 or unexpected non-zero with parsed matches remains successful but partial with `error.category=exec_failed`.

Do not alter unrelated ownership boundaries in ADR 0008 unless the implementation materially changes them, which this plan does not expect.

## 6. Create the Response Artifact

After implementation, tests, and commit are complete, create:

```text
/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md
```

Create the `responses/` directory if needed.

The artifact must include:

- Original request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-26-coding-agent-tool-parity-for-eino-agent.md`
- Verification-gap request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-27-coding-agent-tool-parity-verification-gaps.md`
- Completion status for the original parity request.
- Exact commit or tag for `eino-agent` to pin, from `git rev-parse HEAD` after the final commit.
- Verification commands and results:
  - `go test ./...`
  - `go vet ./...`
  - `golangci-lint run`
- Remaining intentional deferrals, if any. If none, state `None`.

Do not fill in a speculative commit hash before the final implementation commit exists.

## 7. Final Verification and Closeout

Run:

```bash
go test ./...
go vet ./...
golangci-lint run
git status --short
```

Then follow the repo close protocol:

```bash
bd close <implementation-bead-id> --reason="Verification gaps closed"
git add <changed-files>
git commit -m "Close coding-agent parity verification gaps"
git pull --rebase
bd dolt push
git push
git status
```

`git status` must show the branch up to date with origin before the work is considered complete.

