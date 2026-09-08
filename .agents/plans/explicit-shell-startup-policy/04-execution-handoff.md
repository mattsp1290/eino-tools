# Execution handoff

This is an implementation specification. None of the work packages below has been implemented by the planning session. Use Beads to claim and record execution; preserve unrelated work and existing plan artifacts.

## Ordered work packages

| Order | Bounded result | Main change surfaces | Prerequisite and gate |
| --- | --- | --- | --- |
| 1 | Host-owned startup API and real leaf execution | Existing `shell/options.go`, `shell/shell.go`, options/shell tests, `shell/doc.go`; proposed `shell/example_test.go` under `shell` | Confirm baseline and context; `go test -race ./shell` |
| 2 | Captured catalog policy and fingerprint | Existing `catalog/executable_unix.go`, `catalog/catalog_unix.go`, `catalog/identity.go`, catalog tests and ADR 0009 | Package 1; `go test -race ./shell ./catalog`, existing identity goldens retained except shell metadata |
| 3 | Mounted proof, platform coverage and documentation | Proposed nested module `integration/shellstartup/` under repository root; existing CI, README, ADR 0002/0009 and shell docs | Packages 1–2; real published mount adapter tests on Linux/macOS and all root gates |
| 4 | Published usable pin and response | Actual implementation commit and new external response under the existing eino-tools project directory | All tests/CI pass; replacement-free mounted tests using remotely downloadable pin |

Package 4 is the delivery portion of [03-mounted-verification-and-delivery.md](03-mounted-verification-and-delivery.md), not a second implementation design. Implement packages 1–3 in dependency order, preferably one cohesive feature PR. Documentation may be drafted alongside package 2 once API names settle; shared schema goldens and catalog capture files require a single editing owner. Do not publish a partial API while capture/identity remains incomplete. No feature-flag decisions or new upstream feature acceptance are prerequisites.

## First action and operating constraints

Read [01-shell-policy.md](01-shell-policy.md), then inspect `git status` and the current `shell/options.go` before editing. Use `bd prime` and `bd update eino-tools-5b5 --claim` after planning issue `eino-tools-axx` closes. Baseline is `99b7b6adda67`; re-check affected symbols if HEAD changes. New implementation files/symbols are explicitly marked proposed in the work files. Proposed integration paths have the existing repository root as insertion point.

Preserve the user's no-active-users/no-compatibility-required context. Default `-lc`, model schema authority, command fidelity and execution controls remain explicit source-request constraints. No stored-data migration is needed. New shell hashes require new run-plan identity; do not silently migrate or resume incompatible plans. A rollback is an artifact selection for newly constructed plans, not mutation of mounted policy.

## Verification commands and evidence

At root, after implementation:

```sh
go mod tidy -diff
go vet ./...
make lint
go test -race ./...
```

Run all dependency-hygiene checks from existing `.github/workflows/ci.yml`, retain its Windows `go test ./catalog` contract and tracker integration job. Do not silently omit unrelated existing required CI jobs. Root lint is pinned by `Makefile`; installing an older local golangci-lint is not equivalent evidence.

From proposed `integration/shellstartup/`, with Go >=1.26.3:

```sh
GOWORK=off go mod tidy -diff
GOWORK=off go vet ./...
GOWORK=off go test -race -count=1 -timeout=120s ./...
```

The proposed platform CI job runs both focused root packages and this nested module on Linux and macOS. The separate controlled Linux container runs all real login-mode regressions with `EINO_TOOLS_TEST_LOGIN=1`; ordinary host commands leave this test-only opt-in unset. Inspect test output for executed fixtures; skipped startup/mount tests do not pass acceptance. No real HOME profiles, credentials or environment dumps are test evidence. After push, repeat the nested checks with the published full implementation SHA and no replace as specified in package 3.

## Requirement-to-evidence map

| Source requirement | Plan location | Evidence required before implementation completion |
| --- | --- | --- |
| Acceptance 1: published public leaf/catalog API and construction example | Packages 1, 2 and package 3 publication/examples | Actual full pushed implementation SHA, remote module resolution without replace, compiling leaf and mounted example, exact Go and module requirements |
| Acceptance 2: synthetic HOME startup file is not loaded, workspace preserved | Package 1 tests 3–4; package 3 mounted tests 1–3 | Real `/bin/sh` execution, decoded cwd/HOME/canary assertions, absent marker, same-fixture source control produces canary and marker |
| Acceptance 3: default, explicit, invalid, capture/mutation and argument fidelity | Package 1 tests 1–3/7; package 2 tests; package 3 tests 2–5 | Exact argv recorder bytes plus real syntax output, constructor/mount failures, stable behavior after mutation, equal default/login hashes and distinct non-login hash |
| Acceptance 4: timeout, interruption/group cleanup, caps, Env, mounted path | Package 1 tests 5–7; package 3 test 6 | Existing regressions pass, running descendant cleanup verified for timeout and cancellation, both output streams capped, actual mounted executor receives selected policy |
| Acceptance 5: response with API/pin/platform/evidence/limits | Package 3 response | Existing request's same-filename response resolves under documented HOME path and cites verified implementation facts |
| Command remains one argument; model cannot choose host policy | Package 1 execution/schema; package 2 capture; package 3 decoder tests | Quotes/newlines/syntax byte fidelity, unchanged property set, prohibited model fields cannot modify policy |
| Unsupported configuration rejected before execution | Packages 1–2 validation; package 3 mount failure | Unknown mode rejected by leaf and Standard/MountStandard; no partial registration/process launch |
| Immutable captured options and material policy fingerprint | Package 2 identity/capture; package 3 descriptor test | Mode/env/cap mutation cannot change mounted calls; descriptor comparison changes only mode input and detects it |
| Environment/cwd/process controls and precise shell limits | All work packages | Empty Env remains explicit, nil timing preserved, bounded execution, Linux/macOS evidence and documented shell-specific caveats |
| Consumer-owned work and unblock contract | Overview ownership; package 3 response | No Ensemble hook/adoption implementation or issue closure; consumer adoption remains explicitly pending |

## Definition of done and follow-up

The implementation is complete only when all rows above have direct evidence, all packages are committed and pushed, required CI is green, the replacement-free mounted module passes, and the response identifies a usable implementation pin. A planning commit does not satisfy implementation acceptance. Never mark the inbound request implemented during plan delivery.

The plan is ready only after its required two independent reviews and fresh adversarial review complete, accepted findings are incorporated, links and the application-context digest validate, and no blocking decision remains.

Deferred to Ensemble: adopting the new dependency, choosing production policy, analogous workspace-hook startup/lifetime changes, its own synthetic and mounted-runtime acceptance, and closure of `local-symphony-vmxw`/`local-symphony-mdr6`. Deferred outside this change: search's analogous empty-Env capture behavior, tracked separately in Beads `eino-tools-llb`. No new shared repository or external capability request is needed.
