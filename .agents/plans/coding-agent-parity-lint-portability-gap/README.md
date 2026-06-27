# Coding-Agent Parity Lint Portability Gap

Source request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-27-coding-agent-parity-lint-portability-gap.md`

Project: `eino-tools`

Change type: defect closure / verification hardening

## Goal

Make the coding-agent tool parity verification durable across the repo's normal local and CI lint entry points. The parity implementation is already present at HEAD `b48a70fd95502ba00e038946e88b9a8939017534`, but the lint proof in the response artifact depended on a temporary `/tmp/eino-tools-bin` `golangci-lint` binary.

The implementing agent should close two gaps:

1. Plain `golangci-lint run` from the repo root fails in `fileops/read.go` with gosec `G705`.
2. The CI lint step uses `golangci-lint` v2.4.0 through `go run`, which is built with Go 1.24 and cannot load this repo's Go 1.26 lint config.

## Affected Areas

- `fileops/read.go`: numbered read output construction in the line-windowed read loop.
- `.github/workflows/ci.yml`: the `Lint` step in the `unit` job.
- `.golangci.yml`: only if the CI/local tool update reveals a repo-owned config issue; do not lower the configured Go target just to fit an old linter binary.
- `README.md` or a short docs file: document the durable repo-standard lint command once it is added.
- `/Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md`: update if the final pushed commit changes the pin that `eino-agent` should consume.

## Current Signals

Recent history:

- `b48a70f Close coding-agent parity verification gaps`
- `7c933c7 Refine parity verification gap plan`
- `8891f34 Plan coding-agent parity verification gap closure`
- `977f4a8 Add coding-agent tool parity`

Local tool signal from this planning session:

```text
go version go1.26.0 darwin/arm64
golangci-lint has version v2.1.6 built with go1.24.4 ...
```

The request reports another normal PATH binary as `/opt/homebrew/bin/golangci-lint` v2.10.1, and reports that the temporary v2.12.2 binary passes. Treat the exact installed local version as environment-specific; the durable fix should make the repo's documented lint path explicit and reproducible.

## Design Decisions

- Fix the `fileops/read.go` warning by avoiding formatted output on tainted file content. Prefer direct `strings.Builder` writes:
  - `numbered.WriteString(strconv.Itoa(totalLines))`
  - `numbered.WriteString(": ")`
  - `numbered.WriteString(line)`
- Do not suppress `G705` for `fmt.Fprintf` unless direct builder writes fail for a concrete reason. The request explicitly identifies direct builder writes as the likely cleaner fix.
- Align local and CI lint around one durable invocation. Preferred outcome: add a repo-provided lint entry point that builds/runs a pinned `golangci-lint` version with the repo's configured Go 1.26 toolchain, and have CI call that same entry point.
- Do not reduce `go.mod` or `.golangci.yml` from Go 1.26 to work around a stale lint binary. That would weaken the repo baseline and conflict with the current CI matrix.
- Keep this change focused on verification portability. Do not reopen completed parity behavior, search classification, ADR 0008, or tool schema decisions unless a verification command proves they are still broken.
- A downloaded `golangci-lint` release binary is acceptable only if the implementing agent proves `golangci-lint --version` reports a build Go version compatible with `.golangci.yml` `run.go: "1.26"`. If that proof is not available, build the pinned linter from source with the repo's Go toolchain.

## Constraints

- Preserve public APIs, JSON fields, schemas, and tool behavior.
- Keep all changes compatible with Go 1.26.
- Avoid `/tmp`-dependent tooling in verification docs or response artifacts.
- Use Beads for live task state during implementation.
- End state must be committed and pushed per repo session protocol.
