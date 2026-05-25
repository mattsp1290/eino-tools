# Project Planning with Beads

## Agent Instructions

You are an expert software architect creating a comprehensive task breakdown. This task graph will be executed by AI agents working in parallel, coordinated through MCP Agent Mail with file reservations to prevent conflicts.

<quality_expectations>
Create a thorough, production-ready task graph. Include all necessary setup, implementation, testing, and documentation tasks. Go beyond the basics - consider edge cases, error handling, security considerations, and integration points. Each task should be specific enough for an agent to execute independently without ambiguity.
</quality_expectations>

## Project Information

### Links to Relevant Documentation

- **Shared-modules research set** — `~/docs/eino/index.html` (executive summary of the four-module family) and `~/docs/eino/05-shared-repos-proposal.html` § 4 `eino-tools` (the section flagging this module as "stretch tier" with the layout sketch and scope). `~/docs/eino/02-shared-eino-patterns.html` § 4 "Agent tools as a reusable kit" describes the boundary cuts that must happen during extraction. `~/docs/eino/01-current-state.html` describes the existing symphony tool surface. `~/docs/eino/04-integration-plan.html` gives the extraction-ordering context.
- **Existing implementation to lift from** — `~/git/local-symphony/internal/worker/tools/{fileops,search,shell,trackerwrite}/`. Production-grade and already test-seamed; the v1 module is essentially a generalize-and-extract of this code with the `internal/dispatcher` coupling removed.
- **Coupling points to break** — `~/git/local-symphony/internal/dispatcher` (for `dispatcher.ToolOutcome` — replace with in-module `result.Outcome` enum/constants), `~/git/local-symphony/internal/tracker/tracker.go` (for the close-only writer surface needed by the v1 `tracker_write` execution path — lift only `Close`, not the full `TrackerReader / TrackerWriter / Tracker` triad), and the per-tool result envelopes that currently embed the dispatcher enum.
- **Sibling shared module: beads-go** — `~/git/beads-go/` and its plan files at `~/git/beads-go/docs/prompts/beads-go-v1-standalone-go-sdk-for-bd-cli.md` + `~/git/beads-go/docs/prompts/extract-beads-go.md`. For v1 API scope, `beads-go-v1-standalone-go-sdk-for-bd-cli.md` supersedes the older `extract-beads-go.md`; use `extract-beads-go.md` only for historical CLI/install/test context. The `tracker/beads/` sub-package imports `github.com/mattsp1290/beads-go/beads` and adapts `*beads.Client` to this module's close-writer interface. Mirror the planned beads-go-v1 family conventions (MIT, `main`, `docs/adr/`, hand-curated `CHANGELOG.md`, `.golangci.yml` baseline lifted from local-symphony, pre-v1.0 semver, no-marketing README).
- **Sibling shared module: eino-providers** — `github.com/mattsp1290/eino-providers` (planned). Consumers will typically import both; the two modules must agree on the Go-version baseline and on staying telemetry-free (callers wire `agent-otel` around tool invocations). The prompt currently targets Go 1.26, but the task graph must include a preflight gate that reconciles this with local-symphony and beads-go before implementation starts.
- **`cloudwego/eino` upstream** — `github.com/cloudwego/eino`. The `tool.Tool` / `compose.Tool` interfaces the extracted tools implement. Pin to a single minor (target the same baseline `eino-providers` commits to, expected to be v0.8.x).

### Project Description

**`eino-tools`** is a standalone Go module published at `github.com/mattsp1290/eino-tools` that ships drop-in `cloudwego/eino` tool implementations. It is the fourth member of the `mattsp1290` shared-modules family (alongside `codex-auth-go`, `eino-providers`, `agent-otel`, and the sibling `beads-go` SDK) — extracted from `~/git/local-symphony/internal/worker/tools/` so any future eino-based Go agent can drop in the same tool surface instead of copying source.

**Package layout** (each tool is its own sub-package so consumers pull only what they need):

- `result/` — shared `Outcome` enum/constants (`succeeded`, `failed`, `timed_out`, `rejected`) + validation helpers. This replaces `local-symphony/internal/dispatcher.ToolOutcome` inside extracted packages. The model-facing JSON envelopes remain per-tool result structs; retry policy remains per package because categories differ by tool.
- `fileops/` — `Read`, `Write`, `Edit`, `List` as eino `tool.Tool`s. Workspace-rooted; the existing `canonicalizeWorkspace` path-escape boundary moves with the code verbatim.
- `search/` — ripgrep-backed regex search across the workspace, size-capped output. v1 preserves the current `rg --json -e <pattern>` behavior unless an explicit design bead chooses a pure-Go rewrite and includes the corresponding local-symphony schema/prompt migration.
- `shell/` — sandboxed shell execution preserving the current model-facing `cmd` / `timeout_seconds` schema and `sh -lc <cmd>` behavior. Caller-controlled env and output-cap customization should be constructor/options-level (`Options{Env, ShellBinary, OutputCapBytes}`), not a replacement per-call schema. Sandbox wiring (proxy env, iptables egress) is a consumer concern; the symphony deploy infra stays in `local-symphony/deploy/`.
- `tracker/` — close-only writer interface for the v1 `tracker_write` execution path plus a beads adapter sub-package.
  - `tracker/beads/` — imports `github.com/mattsp1290/beads-go/beads` and adapts `*beads.Client` to the close-writer interface. This is a migration from the current CLI-backed local-symphony beads adapter, not a verbatim lift. The model-facing `tracker_write` schema still exposes `comment`, `transition`, `close`, and `link_pr`; v1 preserves the existing `unsupported_op` behavior for non-close ops.

**Boundary cuts vs the symphony copy** (each is a per-tool refactoring task in the graph):

1. Replace every `internal/dispatcher.ToolOutcome` import with `eino-tools/result.Outcome`. The symphony-side consumer keeps a thin `dispatcher.ToolOutcome ↔ result.Outcome` shim during the cutover.
2. Lift only the close writer shape into `eino-tools/tracker` for v0.1: `Close(ctx context.Context, id, reason string) error`. Do **not** lift `core.IssueState` in v0.1 unless a design bead explicitly chooses to promote `Transition`; `toState` stays a parsed field in the `tracker_write` args only for current-schema compatibility and unsupported-op responses.
3. Preserve shell's current `cmd` / `timeout_seconds` schema and `sh -lc` behavior. Replace hard-coded/inherited proxy-env assumptions with caller-supplied construction options, and document the security trade-off honestly as intentional shell execution inside the caller's sandbox.
4. Keep `tracker/beads/` against the **beads-go Go SDK**, not direct `os/exec` of the `bd` CLI. (The symphony tracker adapter currently shells out; the extraction is also a migration to the typed SDK.)

**Scope reframe (verified against code, not the proposal):** The original eino research doc treats `eino-tools` as benefiting both advisor and local-symphony eventually; today only local-symphony has reusable eino ReAct tools to extract. Advisor has MCP tools (`advisor`, `advisor_models`) and a single-shot provider/core stack, but those are out of scope for `eino-tools`. v1 is sized to **one real consumer — local-symphony** — with the explicit, testable acceptance criterion that `local-symphony/internal/worker/tools/{fileops,search,shell,trackerwrite}/` can be deleted and replaced with thin per-tool wrapper files that import the `eino-tools` sub-packages. Advisor adopts later if it grows reusable multi-turn tool use.

**Pre-v1.0 semver caveat:** This module ships under `v0.x` tags. Breaking changes are allowed between minor versions until both consumers (symphony first; advisor if it adopts) are stable on the API and a `v1.0.0` is cut. The `tracker/` package intentionally starts close-only in v0.1; promoting `Comment`, `Transition`, `LinkPR`, or `IssueState` is a later breaking-design point that must be captured in an ADR/changelog entry.

### Technical Stack

- **Language:** Target Go 1.26, pending a required preflight gate. The graph must include a blocking task to reconcile the Go floor across this repo, `~/git/beads-go`'s two planning docs, `~/git/local-symphony`'s current `go.mod` / `.golangci.yml`, and the planned `eino-providers` baseline. Do not treat Go 1.26 as an already-proven repo convention until that gate closes.
- **Module path:** `github.com/mattsp1290/eino-tools`. Each tool is its own importable sub-package: `eino-tools/result`, `eino-tools/fileops`, `eino-tools/search`, `eino-tools/shell`, `eino-tools/tracker`, `eino-tools/tracker/beads`.
- **Core dependencies:**
  - `github.com/cloudwego/eino` (the `tool.Tool` / `compose.Tool` interfaces). Pin to a single minor matching `eino-providers`' baseline (target v0.8.x).
  - `github.com/google/jsonschema-go` (tool argument schemas — already used by the symphony copy).
  - `github.com/mattsp1290/beads-go` (in `tracker/beads/` only — not pulled by other sub-packages).
- **No auth deps, no eino-ext deps, no OTel deps, no logging deps** in any sub-package. Consumers wire telemetry around tool invocations via `agent-otel`; consumers wire auth around their providers via `codex-auth-go` + `eino-providers`.
- **Build/test tooling:** stdlib `go test`, `go vet`, `golangci-lint` (v2.x, baseline lifted from `~/git/local-symphony/.golangci.yml` with paths retargeted).
- **License:** MIT.
- **CI:** GitHub Actions on the reconciled Go baseline. Pipeline: `go mod tidy --diff`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`, per-package dependency hygiene checks, then a dedicated integration job for `tracker/beads/` that installs the real `bd` binary via `npm install -g @beads/bd` (verify the package name against upstream `gastownhall/beads` `package.json` before implementation), verifies `bd --version`, and runs `go test -tags=integration ./tracker/beads/...` against it. This follows the planned beads-go-v1 pattern; beads-go may not have CI files yet.
- **Repo layout** (pre-task-graph):

  ```
  github.com/mattsp1290/eino-tools/
  ├── LICENSE                 (MIT — replace placeholder if present)
  ├── README.md               (one-sentence purpose + one example + consumer link)
  ├── CHANGELOG.md            (hand-curated, starts at Unreleased)
  ├── go.mod                  (reconciled Go baseline; target 1.26 after preflight)
  ├── .golangci.yml           (lifted from local-symphony, paths fixed up)
  ├── .github/workflows/      (single ci.yml — unit + integration jobs)
  ├── result/                 (Outcome enum/constants + validation)
  │   ├── doc.go              (package-level enum stability contract)
  │   ├── outcome.go
  │   └── *_test.go
  ├── fileops/                (Read, Write, Edit, List)
  │   ├── doc.go
  │   ├── fileops.go          (shared workspace boundary + helpers)
  │   ├── read.go
  │   ├── write.go
  │   ├── edit.go
  │   ├── list.go
  │   └── *_test.go
  ├── search/                 (ripgrep-backed regex search)
  │   ├── doc.go
  │   ├── search.go
  │   └── *_test.go
  ├── shell/                  (sandboxed sh -lc execution with constructor options)
  │   ├── doc.go
  │   ├── shell.go
  │   ├── options.go
  │   └── *_test.go
  ├── tracker/                (close-only writer interface)
  │   ├── doc.go
  │   ├── writer.go           (CloseWriter interface)
  │   └── beads/              (beads-go-backed adapter)
  │       ├── doc.go
  │       ├── adapter.go      (wraps *beads.Client to satisfy CloseWriter)
  │       └── *_test.go       (unit tests with a fake close-client interface; integration test under build tag)
  └── docs/
      └── adr/                (0001-result-outcome-enum.md, 0002-shell-options-and-sandbox.md,
                               0003-tracker-close-writer-v0.1.md, ...)
  ```

### Specific Requirements

1. **Acceptance criterion = the deletion PR.** v1 is not done until a PR against `~/git/local-symphony` deletes `internal/worker/tools/{fileops,search,shell,trackerwrite}/` (and the corresponding test files) and replaces each with thin per-tool wrapper files that import the `eino-tools` sub-packages plus a small adapter file that bridges `eino-tools/result.Outcome` ↔ symphony's `internal/dispatcher.ToolOutcome`. All existing local-symphony tests must still pass. This is the v1 success gate and must depend on every unit, integration, import-hygiene, and dependency-isolation check in the generated graph.

2. **No `internal/dispatcher` import in any extracted package.** Every reference to `dispatcher.ToolOutcome` in the symphony copy becomes `result.Outcome` in the extracted module. The symphony-side `dispatcher.ToolOutcome` either becomes a one-line type alias to `result.Outcome` or stays distinct with a trivial conversion in the consumer wrapper — whichever the symphony team prefers at adoption time. The `result/` package is an enum/constants package, not a universal tool-result envelope.

3. **Workspace-rooted security contract preserved verbatim** for `fileops` + `search`: reject paths that escape the configured workspace root using each package's current resolver semantics. `fileops` keeps `canonicalizeWorkspace` plus `resolveExisting` / `resolveWritable`; `search` keeps its own `EvalSymlinks` constructor and resolver unless the search-backend ADR explicitly chooses a rewrite. The path-escape unit tests move with the code and must still pass. Regression tests assert that traversal attempts (`../`, symlink escapes) return `error.category="path_escape"`, absolute paths return `error.category="validation"`, missing paths return `not_found`, and no operation touches the filesystem outside the workspace.

4. **Shell egress is caller-controlled without changing the model-facing schema.** Preserve current args (`cmd`, `timeout_seconds`) and `sh -lc <cmd>` behavior for source compatibility. Add constructor/options-level controls for env, shell binary, and output cap; cwd remains the configured workspace root unless a design bead explicitly changes local-symphony's prompt/schema too. The `#nosec G204` annotation + comment block stays on the exec runner, but the comment must say shell execution of model-supplied `cmd` is intentional and contained by the caller's workspace/container sandbox. Sandbox wiring (squid + iptables egress allowlist) remains a consumer concern; `local-symphony/deploy/` keeps its setup unchanged.

5. **Tracker adapter via beads-go SDK, not direct `os/exec`.** `tracker/beads/` imports `github.com/mattsp1290/beads-go/beads` and adapts `*beads.Client` to `tracker.CloseWriter`. Surface only the operation the symphony `trackerwrite` tool actually executes today (`Close`). `Comment`, `Transition`, `LinkPR`, and `IssueState` are deferred until both the tool and beads-go promote them. **The adapter file has zero `os/exec` import** — the bd CLI invocation lives inside beads-go's transport.

6. **`CloseWriter` interface lives in `eino-tools/tracker`, lifted from the actual v1 execution path only.** Do NOT re-export `local-symphony/internal/tracker`'s full `TrackerReader / TrackerWriter / Tracker` triad — that's a local-symphony-specific abstraction. Define the v0.1 interface as `Close(ctx context.Context, id, reason string) error`. The current model-facing `tracker_write` args still parse `toState`, `body`, and `prURL` so non-close ops can return the existing `unsupported_op` envelope, but those fields do not require public tracker state types in v0.1.

7. **Each tool is its own sub-package with independent imports.** A consumer that only wants `fileops` must not transitively pull `beads-go` (tracker), `cloudwego/eino-ext` (none of the sub-packages should), or `os/exec`. `search` and `shell` may intentionally import `os/exec`; `fileops`, `result`, and `tracker` must not. Verify with `go list -deps` checks for every public package, not only `fileops`, and assert each package's forbidden dependency set explicitly.

8. **No telemetry, no logging.** The library does not import `log`, `slog`, or any `go.opentelemetry.io/...` package in any sub-package. Callers wrap. (Consistent with `agent-otel` being a separate spine module and with beads-go's same constraint.)

9. **Source-compatible tool ABI and schema behavior preserved.** Every extracted tool keeps the current public constants and entry points expected by local-symphony: exact `Name` constants, `Schema() json.RawMessage` returning a fresh copy, `InvokableRun(context.Context, string) (string, error)`, parse-error behavior, duplicate top-level key rejection, and `additionalProperties:false` schemas. Add tests that prove schema mutation does not affect later calls and duplicate top-level JSON keys are rejected before execution.

10. **Forward-compat contract documented at package level** for every per-tool result struct: unknown JSON fields preserved in a `RawJSON json.RawMessage` escape hatch; adding a typed field is non-breaking; renaming or retyping one is breaking. A unit test asserts decoding a payload with an unknown top-level field succeeds and that the unknown field survives in `RawJSON`. `result/` documents the enum stability contract separately.

11. **Pre-v1.0 versioning.** Ships under `v0.x` tags. Breaking changes allowed between minor versions until both consumers stabilize on the API. `CHANGELOG.md` documents every breaking change with a migration note. Later promotion of `tracker.Comment`, `tracker.Transition`, `tracker.LinkPR`, or public `IssueState` is the most likely source of a v0 → v0 breaking change.

12. **CI installs the real `bd` binary via `npm install` for the `tracker/beads/` integration job.** A dedicated GitHub Actions job (separate from the unit-test job) sets up Node, runs `npm install -g @beads/bd` (verify this package name against upstream `gastownhall/beads` `package.json` before implementation), verifies `bd --version` is on `PATH`, then runs `go test -tags=integration ./tracker/beads/...`. Integration tests must create a temp dir, run `bd init` against it, and exercise `Close` end-to-end through the real binary + the real beads-go SDK.

13. **Replay-style fake for tracker unit tests.** Unit tests for `tracker/beads/` use a fake implementing the small local interface the adapter accepts, for example `interface { Close(context.Context, string, string) error }`. A fake cannot implement the concrete `*beads.Client` type. No `tracker/beads/` unit test depends on the real `bd` binary except the integration job in #12.

14. **Preflight gates before extraction work.** The generated graph must include blocking `preflight` beads to: reconcile Go baseline across eino-tools, beads-go, local-symphony, and eino-providers; confirm/pin the `github.com/cloudwego/eino` minor; verify `github.com/mattsp1290/beads-go` is tagged/importable at the needed API surface; and decide whether local-symphony's acceptance PR includes the Go/eino bump or depends on a separate completed PR.

15. **Family conventions (planned from beads-go-v1):**
    - MIT LICENSE in repo root.
    - `main` default branch (no `master`).
    - `docs/adr/` directory carrying design decisions worth preserving — minimum ADRs: `0001-result-outcome-enum.md`, `0002-shell-options-and-sandbox.md`, `0003-tracker-close-writer-v0.1.md`, `0004-tracker-beads-via-sdk-not-exec.md`, `0005-search-backend-ripgrep-vs-pure-go.md`.
    - Hand-curated `CHANGELOG.md` on the way to v1.0.0; switch to `release-please` once tagged.
    - README opens with: one sentence on what it does, one example per tool sub-package, link to consumer projects. No marketing.
    - `.golangci.yml` baseline matching `~/git/local-symphony/.golangci.yml`.
    - GitHub Actions CI on the reconciled Go baseline.

---

## Your Task

Analyze this project and create a comprehensive **Beads task graph** using the `bd` CLI. Beads provides dependency-aware, conflict-free task management for multi-agent execution.

---

<critical_constraint>
Your ONLY output is a bash shell script. Do NOT use `bd add` — the correct command to create a bead is `bd create`. Use `bd dep add` for dependencies. Do not implement anything yourself.
</critical_constraint>

## Output Format

Generate a shell script that creates the full task graph. The script should:

1. **Initialize Beads** (if not already initialized)
2. **Create all beads** with appropriate priorities
3. **Establish dependencies** between beads
4. **Add labels** for phase grouping

### Example Output

```bash
#!/bin/bash
# Project: eino-tools
# Generated: 2026-05-25

set -e

# Initialize beads if needed
if [ ! -d ".beads" ]; then
    bd init
fi

echo "Creating project beads..."

# ========================================
# Phase 1: Project Setup & Infrastructure
# ========================================

SETUP_GOMOD=$(bd create "Initialize Go module at github.com/mattsp1290/eino-tools after Go baseline preflight" \
  -d "Run 'go mod init github.com/mattsp1290/eino-tools'. Set the go directive to the reconciled baseline from the preflight gate. Add cloudwego/eino and google/jsonschema-go as initial deps; beads-go is added later when tracker/beads/ lands." \
  -p 0 -l setup --silent)

SETUP_LICENSE=$(bd create "Replace LICENSE with MIT text, name = Matt Spurlin, year = 2026" \
  -d "Overwrite the existing LICENSE placeholder with the standard MIT template. Year 2026, copyright holder 'Matt Spurlin'." \
  -p 1 -l setup --silent)

SETUP_LINT=$(bd create "Add .golangci.yml mirroring local-symphony's v2.x baseline" \
  -d "Copy ~/git/local-symphony/.golangci.yml, retarget go version to the reconciled baseline, drop any local-symphony-specific path skips." \
  -p 1 -l setup --silent)
bd dep add $SETUP_LINT $SETUP_GOMOD

# ... continue for all phases ...

echo ""
echo "Bead graph created! View with:"
echo "  bd ready              # List unblocked tasks"
```

---

## Bead Creation Guidelines

### Priority Levels
- `-p 0` = Critical (blocking other work)
- `-p 1` = High (important but not blocking)
- `-p 2` = Medium (standard work)
- `-p 3` = Low (nice to have)

### Labels (Phase Grouping)
Use `--label` to group beads by phase:
- `preflight` - Version/source inventory gates before implementation
- `api` - Public API decisions and ADRs
- `setup` - Project initialization
- `core` - Shared outcome/API architecture
- `feature-fileops` - File operation tool extraction
- `feature-search` - Search tool extraction
- `feature-shell` - Shell tool extraction
- `feature-tracker` - Tracker close writer and tracker_write extraction
- `testing` - Test coverage
- `hygiene` - Import boundaries, dependency isolation, no telemetry/logging checks
- `consumer` - local-symphony deletion/wrapper acceptance PR
- `docs` - Documentation
- `deploy` - CI
- `release` - Changelog, README, tag-readiness

### Dependency Rules
1. Never create cycles
2. Every bead should have a clear dependency chain back to setup tasks
3. Use `bd dep add CHILD PARENT` (child depends on parent completing first)
4. Parallel work should share a common ancestor, not depend on each other
5. Required graph gates:
   - `preflight` beads block all implementation beads.
   - `result/` blocks every tool package.
   - fileops path-boundary helpers block fileops read/write/edit/list beads.
   - search-backend ADR blocks search implementation.
   - shell-options ADR blocks shell implementation.
   - tracker close-writer blocks tracker/beads and trackerwrite extraction.
   - tracker/beads depends on a tagged/importable beads-go SDK.
   - CI and hygiene checks depend on all package implementations.
   - local-symphony deletion PR depends on unit tests, integration tests, and hygiene checks.

### Task Granularity
- Each bead should be completable in **under 750 lines of code**
- Tasks should be atomic enough for one agent to complete without coordination
- If a task requires multiple file areas, consider splitting by file area

---

## File Reservation Planning

For each major work area, note the file patterns that will need exclusive reservation:

```bash
# Example reservation notes (add as bead descriptions)
# Preflight/version gates: docs/prompts/** only, no source writes
# Result outcome enum: result/**
# fileops tool: fileops/** (includes shared canonicalizeWorkspace helper)
# search tool: search/** (ripgrep-backed unless ADR chooses rewrite)
# shell tool: shell/** (includes constructor options)
# Tracker close writer interface: tracker/{writer,doc}.go, tracker/*_test.go
# Tracker beads adapter: tracker/beads/**
# ADRs: docs/adr/**
# CI / lint config: .github/workflows/**, .golangci.yml
# Integration tests: tracker/beads/*_integration_test.go (build tag: integration)
# Consumer-side acceptance PR: lives in ~/git/local-symphony, not this repo
```

This helps agents claim appropriate file surfaces when they start work.

---

## Context Documentation

Place any important context in `prompts/docs/` for agents to reference. This includes:
- Architecture decisions
- API documentation
- Design system specs
- External service integration guides

---

## Verification Steps

After generating the script:

1. **Run it**: `chmod +x setup-beads.sh && ./setup-beads.sh`
2. **Check ready work**: `bd ready` should show initial setup tasks

---

## Completeness Checklist

Ensure your task graph includes:

- [ ] All setup and configuration tasks
- [ ] Core architecture and shared utilities
- [ ] Preflight gates for Go/eino/beads-go baselines
- [ ] ADRs for result enum, shell options, tracker close writer, beads SDK adapter, and search backend
- [ ] Feature implementation tasks (broken into small units)
- [ ] Source-compatible tool ABI and schema tasks
- [ ] Error handling and edge cases
- [ ] Unit and integration tests for each feature
- [ ] Per-package dependency isolation and no telemetry/logging checks
- [ ] local-symphony deletion/wrapper acceptance PR gate
- [ ] API documentation
- [ ] Security considerations (input validation, auth checks)
- [ ] Performance considerations where relevant
- [ ] CI/CD and deployment tasks
- [ ] Clear dependency chains with no cycles
