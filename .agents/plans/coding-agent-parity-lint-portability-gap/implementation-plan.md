# Implementation Plan

Use this file as the execution guide for the coding agent. Track actual work with `bd`; do not use this markdown file as a live task tracker.

## 1. Start and Baseline

Run from `/Users/punk1290/git/eino-tools`:

```bash
bd ready
bd create --title="Fix coding-agent parity lint portability gap" --description="Make local and CI lint verification durable for the coding-agent parity implementation without relying on /tmp tooling." --type=bug --priority=0
bd update <id> --claim
```

Then capture the baseline:

```bash
go test ./...
go vet ./...
golangci-lint --version
golangci-lint run
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0 run ./...
```

The last two commands are expected to fail as described in the request. If `go test` or `go vet` fails, stop and determine whether the failure is unrelated drift before editing.

## 2. Fix `fileops/read.go` G705

In `fileops/read.go`, replace the numbered output formatting call:

```go
fmt.Fprintf(&numbered, "%d: %s", totalLines, line)
```

with direct builder writes:

```go
numbered.WriteString(strconv.Itoa(totalLines))
numbered.WriteString(": ")
numbered.WriteString(line)
```

Then update imports:

- Add `strconv`.
- Keep `fmt`; it is still used elsewhere in `fileops/read.go` for error messages.

Run the focused package tests:

```bash
go test ./fileops
```

This change should preserve exact output. Existing read-window tests should catch newline, prefix, and truncation regressions.

## 3. Make CI Lint Durable

Update `.github/workflows/ci.yml` so the `Lint` step no longer runs the stale v2.4.0 command:

```yaml
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0 run ./...
```

Preferred implementation:

```yaml
- name: Lint
  uses: golangci/golangci-lint-action@v9
  with:
    version: v2.12.2
    args: run ./...
```

Rationale:

- The request already reports v2.12.2 as passing locally when built with Go 1.26.
- The official action gives CI an explicit pinned linter version instead of depending on whatever Go version built an old module binary.
- `args: run ./...` keeps the lint target equivalent to the old CI command.

If the action version or linter version must be adjusted, keep these properties:

- The action major is current for the runner environment, or is a supported major that can run `golangci-lint` v2.
- The linter version is pinned.
- It is a v2.x `golangci-lint`.
- It can load `.golangci.yml` with `run.go: "1.26"`.
- The invocation can be reproduced by a future agent outside CI without relying on `/tmp`.

Do not change the Go version matrix to work around the old linter.

## 4. Document the Repo-Standard Lint Command

If CI uses a pinned action while local development still uses `golangci-lint run`, add a short note to `README.md` near the setup or development commands:

````markdown
### Verification

Run from the repo root:

```bash
go test ./...
go vet ./...
golangci-lint run
```

`golangci-lint` must be a v2.x build compatible with the Go version in `go.mod` and `.golangci.yml` (currently Go 1.26). CI pins this through `.github/workflows/ci.yml`.
````

If the implementation instead adds a repo script such as `scripts/lint.sh`, document that as the standard local command and have CI call the same script. Avoid adding a script unless it materially reduces ambiguity; a pinned action plus README note is likely enough.

## 5. Verify

Run:

```bash
go test ./...
go vet ./...
golangci-lint run
```

Then verify the CI lint command shape locally as far as practical:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 version
```

If `go run ...@v2.12.2 run ./...` still fails because the module binary is built with a lower Go toolchain, do not use that as the repo-standard command. Prefer the GitHub Action for CI and document local binary requirements.

## 6. Update the External Response Artifact if the Pin Changes

If the implementation creates a new commit after `b48a70fd95502ba00e038946e88b9a8939017534`, update:

```text
/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md
```

The artifact should record:

- The new final `git rev-parse HEAD` pin after rebase.
- `go test ./...`: passed.
- `go vet ./...`: passed.
- `golangci-lint run`: passed with the normal documented local command.
- CI lint portability fix: stale v2.4.0 command replaced with a pinned compatible v2.x linter path.

Do not write a speculative pin before the final commit exists.

## 7. Closeout

Follow the repo protocol:

```bash
git status --short
git add <changed-files>
git commit -m "Fix coding-agent parity lint portability"
git pull --rebase
PIN=$(git rev-parse HEAD)
# Update external response artifact now if needed.
bd close <id> --reason="Lint portability gap fixed"
bd dolt push
git push
git status
```

The final `git status` must show the branch up to date with origin.
