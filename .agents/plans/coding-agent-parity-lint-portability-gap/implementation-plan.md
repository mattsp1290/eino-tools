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

Preferred implementation: add a repo-local lint script and have CI call it.

Create `scripts/lint.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${GOLANGCI_LINT_VERSION:-v2.12.2}"
bin_dir="${GOLANGCI_LINT_BIN_DIR:-${repo_root}/.cache/tools/bin}"
bin="${bin_dir}/golangci-lint-${version}"

mkdir -p "$bin_dir"
if [ ! -x "$bin" ]; then
  GOBIN="$bin_dir" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${version}"
  mv -f "${bin_dir}/golangci-lint" "$bin"
fi

"$bin" --version
cd "$repo_root"
"$bin" run ./...
```

Make the script executable and add `.cache/` to `.gitignore` if it is not already ignored. This keeps the lint binary out of `/tmp`, pins the version, and builds the linter with the Go toolchain selected by the caller. CI already uses `actions/setup-go@v6` with Go 1.26.x before lint, so the script-built linter is compatible with `.golangci.yml` `run.go: "1.26"`.

Then change the CI `Lint` step to:

```yaml
- name: Lint
  run: ./scripts/lint.sh
```

Rationale:

- The request already reports v2.12.2 as passing locally when built with Go 1.26.
- Building the pinned linter inside the selected Go 1.26 CI environment avoids the old failure mode where `golangci-lint` was built with Go 1.24 or Go 1.25 and refused the Go 1.26 config.
- The same script is a durable local command for future agents: `./scripts/lint.sh`.
- The resolved lint command remains equivalent to `golangci-lint run ./...`.

Alternative implementation: use `golangci/golangci-lint-action` only if the implementer proves the actual action-installed binary is compatible.

If choosing the action instead of the script:

- Use an exact, verified action release tag, not the mutable `@v9` major tag.
- Pin the linter version, preferably through a committed `.golangci-lint-version` or `.tool-versions` file consumed by `version-file`.
- Do not pass `args: run ./...`; the action already runs the `run` subcommand. Use `args: ./...` or omit `args` if the default target is sufficient.
- In CI logs, verify the action-installed `golangci-lint --version` reports a build Go version compatible with Go 1.26 before treating the action path as accepted.

Do not change the Go version matrix to work around the old linter.

## 4. Document the Repo-Standard Lint Command

Add a short note to `README.md` near the setup or development commands:

````markdown
### Verification

Run from the repo root:

```bash
go test ./...
go vet ./...
./scripts/lint.sh
```

`./scripts/lint.sh` builds a pinned `golangci-lint` v2.x binary with the active Go toolchain, then runs `golangci-lint run ./...`. Use Go 1.26 or newer so the linter can load this repo's `.golangci.yml`.
````

Plain `golangci-lint run` may still be mentioned as acceptable when the installed binary is already a compatible v2.x build, but the script should be the durable repo-standard command.

## 5. Verify

Run:

```bash
go test ./...
go vet ./...
./scripts/lint.sh
```

Also run plain lint if a normal PATH binary is available:

```bash
golangci-lint --version
golangci-lint run
```

If plain `golangci-lint run` fails only because the PATH binary was built with an older Go version, the acceptance path is still satisfied by the durable documented `./scripts/lint.sh` command. If it fails with real lint findings, fix those findings.

## 6. Update the External Response Artifact if the Pin Changes

If the implementation creates a new commit after `b48a70fd95502ba00e038946e88b9a8939017534`, update:

```text
/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md
```

The artifact should record:

- The new final `git rev-parse HEAD` pin after rebase.
- `go test ./...`: passed.
- `go vet ./...`: passed.
- `./scripts/lint.sh`: passed with the durable documented local command.
- `golangci-lint run`: passed if the normal PATH binary is compatible; otherwise note that the repo-standard script passed and explain the stale PATH binary.
- CI lint portability fix: stale v2.4.0 command replaced with the repo-standard pinned lint script.

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
