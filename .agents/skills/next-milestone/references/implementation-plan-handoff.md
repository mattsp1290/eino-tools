# Resolved Milestone Brief and Planning Handoff

Use this reference after the user selects a candidate and no upstream request blocks it. Keep the brief in conversation context; it is not a separate artifact.

## Brief schema

Populate every field. Use `None` only when evidence proves irrelevance and preserve uncertainty.

```markdown
# Resolved milestone brief

## Selection
- Selected milestone:
- Planning status: ready | blocked
- Suggested kebab-case plan name:
- Primary consumer and observable agent outcome:
- Milestone type: add tool | strengthen contract | improve delivery
- Selection rationale or accepted non-recommended trade-off:

## Scope
- In-scope tool/contract journey:
- Out of scope:
- Supported platform and executable boundary:
- Explicit Pi/OpenCode parity-claim boundary:

## Evidence
- Eino-tools facts and exact paths/symbols/tests:
- Existing-plan and Beads relationship:
- Local dependency facts, revisions, public contracts, and pins:
- External facts, direct URLs, revisions/dates, publishers, and access dates:
- Inferences:
- Proposals:

## Architecture and behavior
- Relevant capability gaps and comparator lessons:
- Current metadata/construction/execution/result/consumer flow:
- Candidate before/after flow:
- Proposed ownership, packages, public API, schema, result, catalog, and consumer integration:
- Tool and executor identity, configuration, provenance, concurrency, and capabilities:

## Requirements and gates
- Functional and non-functional requirements:
- Dependencies and execution order:
- Deep-dive component:
- Cancellation, deadline, timeout, retry, and partial-failure behavior:
- Path/process/network/environment/permission and secret behavior:
- Input/output/resource bounds, text or multimodal result transport, truncation, and artifact behavior:
- Compatibility, migration, rollback, deprecation, and removal behavior:
- Supply-chain and external-executable behavior:

## Decisions and requests
- User decisions:
- Assumptions:
- Unresolved upstream requests: none
- Resolved request/response evidence and verified pin:
- Blocking open questions, owner, and exact unblock action:
- Non-blocking open questions:

## Verification
- Unit, integration, race, fuzz, schema ABI, and consumer tests as applicable:
- Credential-free acceptance path:
- Optional configured-service smoke path:
- Observable acceptance criteria:
```

Planning status is `ready` only when no material architecture, behavior, compatibility, security, or upstream decision remains open.

## Resume `$implementation-plan`

For a composed invocation:

1. Ask and record `$implementation-plan`'s required operating-context questions; do not answer them by inference.
2. Derive one safe kebab-case plan name and create exactly one direct child under `.agents/plans/`.
3. Incorporate the brief into the normal plan files before review; do not write it as an extra artifact.
4. Retain every standard structure, grounding, review, revision, and delivery requirement.
5. If decision-blocked, preserve that status, owner, and unblock action and do not call the plan implementation-ready.

The plan must additionally contain the affected capability-gap table; current and before/after tool flow; current Pi and OpenCode citations; verified Eino versions and contracts; leaf-versus-host ownership; schema, result, identity, provenance, bounds, permissions, cancellation, retry, concurrency, and compatibility decisions; relationships to planned-but-unimplemented work; captured user choices; and bounded verification through the real public consumer path when consumer readiness is claimed.

If an upstream request is unresolved, do not resume `$implementation-plan`.

## Standalone behavior

When `$next-milestone` runs alone, return the populated brief after selection and end with:

```text
Use $implementation-plan with this resolved milestone brief as the concrete request to create the repository's reviewed implementation plan.
```

If the user later invokes the composed form, refresh request and current-evidence gates but preserve the selection unless new evidence invalidates it.
