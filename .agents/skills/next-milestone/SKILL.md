---
name: next-milestone
description: Choose the next eino-tools milestone by comparing the live reusable Go tool library and local Eino contracts with current Pi and OpenCode coding-agent tool behavior; present bounded candidates for user selection; create and block on cross-repository requests when required upstream contracts are missing; and hand a resolved milestone to $implementation-plan. Use when asked what tool capability to build next in eino-tools or when invoked as $implementation-plan $next-milestone.
---

# Next Eino Tools Milestone

Choose one bounded, reusable coding-agent tool increment for this repository, then let `$implementation-plan` plan it. Re-evaluate the repository, local Eino ecosystem, and external references on every invocation; none is a frozen roadmap.

## Ownership and ordering

When invoked as `$implementation-plan $next-milestone`, preserve this order:

1. Let `$implementation-plan` resolve the repository root and applicable guidance, but do not let it name or create a plan directory yet.
2. Run this skill's outstanding-request resume gate. If it does not stop the run, survey the repository and local Eino contracts, research current Pi and OpenCode behavior, compare the frontier, present candidates, and pause for the user's choice. If this conversation already contains a resolved brief from this skill, refresh its evidence and preserve its selection unless new facts invalidate it.
3. Resolve the selected milestone, material decisions, end-to-end tool flow, and component ownership.
4. If the selected milestone needs a missing public contract from another local repository, follow the upstream-request protocol, report every blocker, and stop. Do not create an eino-tools plan while any request remains unresolved.
5. Otherwise, build the resolved milestone brief and resume `$implementation-plan`, including its user-confirmed operating context, normal plan format, and required reviews.

`$next-milestone` owns milestone selection and upstream-request gating. `$implementation-plan` owns plan naming, files, review, revision, and delivery. Blocking requests are the only artifacts this skill may create. Never create an interim plan, `/big-change` prompt, Beads issue, implementation change, or second plan format.

When invoked alone, complete discovery, selection, decisions, and either the resolved brief or blocking request set. Do not write an implementation plan.

## Workflow

### 1. Establish the live frontier

Resolve the Git root and read applicable `AGENTS.md` and contributor guidance. Read [references/research-playbook.md](references/research-playbook.md), then inspect the current worktree, history, plans and requests, packages, public schemas and constructors, catalog registration, tests, examples, CI, releases, and quality state.

Before ordinary discovery, scan `~/.agents/projects/*/requests/` for records whose `Blocker consumer` is `eino-tools`. Evaluate every match using [references/upstream-request-protocol.md](references/upstream-request-protocol.md). If a request remains open or its claimed resolution cannot be verified, report every blocker and stop before refreshing candidates. Do not evade a blocker by selecting a replacement milestone unless the user explicitly withdraws or supersedes the blocked milestone and the request records that transition.

Build a shallow inventory of every unique resolved `~/git/eino-*` Git root, including `eino-agent` and excluding this repository. Read relevant repository guidance and deep-inspect only siblings connected to affected capability lanes. Treat sibling checkouts as contract and ownership evidence, never permission to modify them. Preserve unrelated and uncommitted work everywhere.

Distinguish executable, tested tool behavior from scaffolds, metadata-only registration, plans, docs, and stale tracker state. Never read secret-bearing environment, credential, session, or prompt-content files.

### 2. Research current references

Browse on every unblocked candidate-selection invocation using the mandatory lanes in the research playbook.

- Pi's official coding-agent tools and extension API are the minimal, extensible comparator.
- OpenCode's official tools, permission model, and coding workflows are the feature-rich comparator.
- Current `eino-agent` public composition, tool execution, permission, persistence, and event contracts are the primary consumer authority.
- Eino and each proposed Go or executable dependency provide implementation constraints.

Record direct URLs, inspected revisions when available, and access dates. Treat comparator behavior as product and architecture evidence, not code to port or an API specification. Preserve license and clean-room boundaries.

If mandatory browsing or authoritative coverage is unavailable, finish the local survey, label volatile claims `unverified-current`, name the missing lanes, and stop before declaring a candidate plan-ready. A user may accept a repo-only exploratory brief, but that does not make external claims current.

### 3. Compare and build candidates

Read [references/frontier-rubric.md](references/frontier-rubric.md). Build its capability matrix and end-to-end tool flow. Keep these evidence classes distinct: `Repo fact`, `Local dependency fact`, `External fact`, `Inference`, `Proposal`, and `User decision`.

Present 2–4 coherent candidates, recommendation first. Every candidate type must be exactly `add tool`, `strengthen contract`, or `improve delivery`; readiness is separate. Show fully planned or already implemented outcomes outside the selectable list. Exclude duplicate, request-only, plan-maintenance, comparator-porting, and purely cosmetic options.

Candidates must differ in consumer value, capability lane, or dependency trade-off. Prefer thin reusable journeys that mount through supported public contracts and prove observable model-facing behavior. A package skeleton, schema alone, catalog entry alone, or mock-only path does not establish coding-agent parity.

### 4. Pause for the user's choice

For each fresh selection, show the 2–4 options and stop before writing a request or plan. This applies even if the initial request says “pick for me.” Use structured user input when available; otherwise ask one concise plain-text question with numbered options. Only after options are visible may the user delegate the choice.

Preserve a non-recommended selection and explain its trade-off without overriding it.

### 5. Resolve decisions and ownership

After selection, ask 1–3 short questions per interaction only when answers materially change observable behavior, public API, safety authority, compatibility, persistence, platform support, external dependencies, or scope. Continue focused rounds while independent material decisions remain.

Trace every required capability to a verified owner. The expected direction is that `eino-tools` owns reusable leaf-tool schemas, execution mechanics, bounded results, metadata, and catalog factories; `eino-agent` owns orchestration, admission, permissions, concurrency coordination, sessions, and model/runtime integration; other Eino siblings retain their established provider, protocol, UI, extension, and observability contracts. Verify this division against current code.

If a required public contract is absent or ambiguous upstream, read [references/upstream-request-protocol.md](references/upstream-request-protocol.md). Create or reuse one blocker request per distinct target-owned contract, mark the milestone `blocked upstream`, report every request and exact unblock condition, and stop. Do not hide the gap behind a duplicate implementation, private import, speculative adapter, or mock presented as completion.

### 6. Hand off to planning

When no upstream request blocks the milestone, read [references/implementation-plan-handoff.md](references/implementation-plan-handoff.md) and build the complete brief in conversation context. Mark it `ready` only when material decisions are resolved; otherwise mark it `blocked` with the decision owner and exact unblock action.

For a composed run, incorporate the brief into the normal `$implementation-plan` files before review. Do not add it as a separate artifact or extra reviewer prompt. Derive the plan name only after selection and create exactly one selected-milestone plan.

## Invariants

- Require exact repository evidence for `implemented` and `partial`; plans and open or stale Beads prove only intent.
- Adapt observable tool behavior and architecture lessons from Pi and OpenCode; do not translate TypeScript modules, internal schemas, prompts, or private boundaries literally into Go.
- Keep model-facing schemas stable, explicit, and provider-neutral. Host policy, credentials, workspace admission, permission decisions, global scheduling, and sandbox provisioning remain host concerns unless a verified public contract says otherwise.
- A working-tool claim must prove metadata/schema → host construction or catalog materialization → validated invocation → bounded execution → stable result envelope, including applicable cancellation and error behavior. A consumer-parity claim additionally requires mounting through the real public `eino-agent` path.
- Treat path containment, symlink races, subprocess trees, environment inheritance, network access, cancellation, timeouts, output limits, retries, idempotency, concurrency declarations, executable provenance, and forward-compatible result decoding as first-class when affected.
- Prefer current public Eino-family APIs. A missing upstream seam is a request/blocker, not authorization to modify a sibling repository.
- Never expose credentials, auth tokens, secret configuration, raw private reasoning, live prompt/session content, or sensitive tool payloads through research notes, requests, logs, plans, or fixtures.
- Narrow “match every coding-agent tool” requests to one reusable capability or contract journey; return a scope blocker if the user declines.
