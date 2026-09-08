# Explicit shell startup policy

Status: Ready — two independent reviews and one fresh adversarial review completed; accepted findings incorporated.

Planning is the deliverable. No implementation, new capability, release, or consumer adoption has occurred in this planning session. Track planning with Beads `eino-tools-axx` and implementation with `eino-tools-5b5`; use Beads for execution status rather than this document.

## Application context

```json
{
  "application_context": {
    "has_active_users": false,
    "backward_compatibility_required": false,
    "feature_flags": "not-applicable",
    "confirmation_digest": "c604d35ed28370417071dce7332f01502078182824f58fafd23843f94b59767e",
    "confirmed_at": "2026-09-08T15:06:20Z"
  }
}
```

The user answered “1. no 2. no” to active consumers and compatibility requirements on 2026-09-08. Both answers are false, so feature flags are not applicable. The named Ensemble integration is evidence of intended consumption, not a contradiction of the user's active-user answer. Retaining the existing default is still an explicit constraint in the source request and a design decision here. No data migration or staged feature rollout is needed.

## Outcome and scope

Change type: additive host configuration with execution-policy identity and integration verification. Affected areas: `shell`, `catalog`, shell documentation, and CI. Hosts can select the equivalent of `/bin/sh -c <command>` using the existing leaf constructor and `catalog.Options.ShellOptions`, including an eino-agent standard mount. Omitted policy continues to invoke `sh -lc <command>`.

Success means that real leaf and mounted executions skip a synthetic HOME profile, expose neither its canary nor its marker, use the admitted workspace, preserve command bytes as one argument, and retain timeout, cancellation, environment and output behavior. Distinct startup modes must have distinct executor identities; caller mutations after capture must not change a mounted policy. A published full implementation commit must be consumable without a fork, module-cache edits, or an argument-translating wrapper.

Out of scope: Ensemble workspace hooks, its dependency adoption and production policy selection, OS containment, environment allowlist design, interactive shell sessions, an arbitrary invocation-argument API, a new registry, and production changes to eino-agent. Do not close Ensemble `local-symphony-vmxw` or `local-symphony-mdr6` as part of this upstream work.

## Verified repository facts

- Baseline: `99b7b6adda67` on branch `explicit-shell-startup-policy`, initially clean and synchronized with its origin branch. The full baseline is resolvable with `git rev-parse 99b7b6adda67`. The preceding `63a3c99` research differs only by the merged file-read fix; preserve that change and its revision 2 identity.
- [Options](../../../shell/options.go) exposes `Env`, `ShellBinary`, and `OutputCapBytes`; `withDefaults` copies nonnil Env without losing explicit emptiness. [Run](../../../shell/shell.go) hardcodes `-lc`, sets workspace cwd, empty stdin, capped streams, timeout, process group, group kill and bounded wait delay. Metadata also hardcodes `sh -lc`.
- [Catalog capture](../../../catalog/executable_unix.go) snapshots normalized environments, invocation paths, executable targets and hashes. Its shell Env copy uses `append([]string(nil), input.Env...)`, losing the distinction between an explicit empty slice and nil before inheritance resolution.
- [Standard](../../../catalog/catalog_unix.go) captures once and calls `shell.New` with the captured options at workspace materialization. Shell executor revision is 1. [Identity](../../../catalog/identity.go) has executable and environment inputs, but no explicit shell policy input.
- [CI](../../../.github/workflows/ci.yml) runs root module tidiness, vet, lint, race tests, dependency hygiene, a Windows catalog contract, and tracker integration. The catalog cannot import eino-agent. Root `go.mod` declares Go 1.26 and Eino v0.8.13; `Makefile` pins lint v2.12.2 with Go 1.26.0.
- Published `github.com/mattsp1290/eino-agent@v0.3.3` was downloaded and inspected. It resolves to `36fe8d8a046b4dd193e97b8f49a580a71bf07bbc`, requires Go 1.26.3, and its `tools/einotools.MountStandard` forwards `Options.Catalog` to `catalog.Standard`. Its integration test demonstrates `AcquireRunPlan`, `ResolveTools`, and the resolved executor. Resolve this source with `go mod download -json github.com/mattsp1290/eino-agent@v0.3.3` and read the returned `Dir`; never hardcode a machine cache path.

## Design decisions and flow

All API additions below are proposed, not existing. Use a string `shell.StartupMode` with empty/default, `StartupModeLogin` (`login`) and `StartupModeNonLogin` (`non-login`). Normalize empty to login. Add `Options.StartupMode` and a portable `Options.Validate() error` so leaf and catalog reject unknown modes consistently before launching a process. No feature flag is added.

Before: host options → catalog capture → leaf tool → fixed `["-lc", command]`.

After: host mode → shared validation/defaulting → captured scalar policy and executor identity → leaf tool → `["-lc", command]` or `["-c", command]`. Model arguments remain only `cmd` and `timeout_seconds`. The selected binary must implement those flags; this is an invocation policy, not a shell detector or sandbox.

Add an optional, shell-specific policy record to executor identity, covering normalized startup mode and effective output cap. Including the cap avoids claiming a complete captured shell policy while omitting a known behavioral option. Keep unrelated hashes unchanged through omission of the new optional field; bump shell revision to 2. Correct policy-neutral shell metadata and deliberately regenerate only its schema golden.

Use a new, separate integration module under the repository root to test the real published mount adapter against this checkout. Its local replace is test wiring, never the consumer's release pin. A second test run against the published implementation commit must remove that replacement. Details and lifecycle cleanup are in work package 3.

## Source request and ownership

Canonical input: `$HOME/.agents/projects/eino-tools/requests/2026-09-08-explicit-shell-startup-policy.md`. Resolve `$HOME` normally; the file already exists and remains unchanged. Owner: eino-tools maintainer. Intended consumer: Ensemble through eino-agent standard catalog mounts, verified in Ensemble's `internal/worker/einoagent/toolregistry.go` at `3f6e92533820657839688dfd225450de795b887f`.

The request drives all work packages. It is an inbound requirement, not a missing upstream dependency blocking local implementation. No additional repository or outbound capability request is needed: this belongs to the existing shell owner, and the adapter API is already published. The request remains open until the implementation response establishes the usable public pin and acceptance evidence. Its new response file belongs at `$HOME/.agents/projects/eino-tools/responses/2026-09-08-explicit-shell-startup-policy.md`; create the `responses` child under the existing project directory only during implementation handoff. Ensemble owns final adoption, hook policy and its own acceptance closure.

## Risks, assumptions and gates

- Non-blocking platform assumption: support the existing Unix build boundary; validate Linux and macOS `/bin/sh` for non-login execution. Other Unix platforms retain compile policy but are unverified until tested. Non-Unix catalog continues returning `ErrUnsupportedPlatform`.
- Shell-specific startup behavior is not universal. Bash invoked under its own name can read `BASH_ENV` during noninteractive execution, whereas noninteractive Bash invoked as `sh` has different rules. Host allowlists and executable provenance still matter. See the [GNU Bash startup manual](https://www.gnu.org/software/bash/manual/html_node/Bash-Startup-Files) and [invocation manual](https://www.gnu.org/software/bash/manual/html_node/Invoking-Bash.html). Do not read real developer startup files to test these rules.
- Publication gate: all acceptance tests, CI and replacement-free module resolution must pass before claiming a usable response. New shell schema/executor identity intentionally invalidates old captured identity; hosts must rebuild plans rather than bypass drift checks.
- No unresolved product decision blocks implementation. Implementation failures in shell fixtures, platform jobs, or published-pin consumption block delivery and belong to the eino-tools implementer to fix. Do not label missing evidence as a pass.

## Document map

1. [01-shell-policy.md](01-shell-policy.md): public mode, validation, execution and leaf regressions.
2. [02-catalog-capture-and-identity.md](02-catalog-capture-and-identity.md): immutable policy, environment ownership and identities.
3. [03-mounted-verification-and-delivery.md](03-mounted-verification-and-delivery.md): real mount acceptance, platform CI, examples and published response.
4. [04-execution-handoff.md](04-execution-handoff.md): ordering, commands, acceptance traceability and completion gates.
