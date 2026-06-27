# Verification Checklist

Use this as a handoff checklist after implementation. The actual source of truth for task state is Beads.

## Required Commands

Run from `/Users/punk1290/git/eino-tools`:

```bash
go test ./...
go vet ./...
./scripts/lint.sh
```

All three must pass using normal repo-root execution. `./scripts/lint.sh` must not depend on `/tmp/eino-tools-bin` or any other temporary binary path.

Also run this when a normal PATH binary is available:

```bash
golangci-lint --version
golangci-lint run
```

Plain `golangci-lint run` may fail only if the installed binary is stale or built with a Go version lower than the configured Go target. If it reports real lint findings, those findings must be fixed.

## Required Code Assertions

- `fileops/read.go` still emits numbered read output in exactly the same format: `<line-number>: <line-content>`.
- The G705 finding at the numbered output write is gone without a broad `//nolint:gosec` suppression.
- No public schemas, exported names, JSON fields, or tool behavior changed.

## Required CI Assertions

- `.github/workflows/ci.yml` no longer uses:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0 run ./...
```

- CI lint calls the durable repo lint entry point, preferably `./scripts/lint.sh`, or otherwise uses a pinned v2.x `golangci-lint` path proven compatible with `.golangci.yml` `run.go: "1.26"`.
- The resolved CI lint command shape remains equivalent to `golangci-lint run ./...`; it must not become `golangci-lint run run ./...`.
- If `golangci/golangci-lint-action` is used instead of the script, use an exact action tag and prove the action-installed binary's `--version` reports a build Go version compatible with Go 1.26.

## Documentation Assertions

- The repo docs identify `./scripts/lint.sh` as the durable local lint command.
- If plain local lint requires a minimum compatible `golangci-lint` build, the repo docs say so.
- Any documented local lint command is durable and reproducible by future agents.
- No documentation tells agents to prepend `/tmp/eino-tools-bin` to `PATH`.

## Response Artifact Assertion

If the repo commit changed after the previous response pin `b48a70fd95502ba00e038946e88b9a8939017534`, update:

```text
/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md
```

It should record the final pushed commit and note that lint now passes through the durable repo-standard path.

## Closeout

Before ending the implementation session:

```bash
git status --short
git add <changed-files>
git commit -m "Fix coding-agent parity lint portability"
git pull --rebase
bd dolt push
git push
git status
```

The final `git status` must show the branch up to date with origin.
