# Eino Tools Frontier Rubric

Use this reference after collecting repository, local-dependency, and current web evidence.

## Capability matrix

Assess relevant lanes independently. Record state, repository evidence, owner/dependency, Pi evidence, OpenCode evidence, gap, and next-milestone relevance.

| Lane | Questions to test |
| --- | --- |
| File discovery and reading | Glob/search/read semantics, line windows, binary and nonregular files, images/PDFs and other model-visible attachments, ignores, Unicode, limits, media detection, path containment, symlink races, executable dependencies |
| File mutation | Write/edit/patch/move/delete semantics, atomicity, optimistic checks, patch grammar, partial failure, permissions, rollback, path safety |
| Shell and processes | Shell startup, cwd/env/stdin, PTY/background needs, cancellation and process trees, timeouts, output caps/artifacts, signals, platform support, sandbox boundary |
| Code intelligence | Language-server lifecycle, symbols, references, diagnostics, formatting, workspace indexing, cancellation, stale results, owner boundary |
| Web and remote content | Fetch/search semantics, URL policy, redirects, size/type limits, freshness, citations, credentials, SSRF/private network boundaries, ownership |
| Agent coordination | Task/subagent tools, delegation state, cancellation, result delivery, concurrency, durable identity, host/runtime ownership |
| User interaction and permissions | Questions, pending/resume behavior, allow/deny/ask policy, normalized arguments, scope, persistence, auditability, host ownership |
| Tracking and planning | Tracker operations, typed state transitions, idempotency, comments, dependency graph operations, SDK versus executable boundary |
| Results and compatibility | Stable envelopes, text versus typed/multipart content, media types and model-visible attachments, outcome vocabulary, unknown-field preservation, truncation/artifact references, schema ABI, errors, migrations |
| Catalog and composition | Deterministic inventory/order, stable identity, fresh factories, platform admission, capability metadata, configuration fingerprints, consumer mounting |
| Safety and concurrency | Leaf versus host guarantees, canonical workspace, shared serialization, retries, exactly-once effects, redaction, resource bounds, race behavior |
| Delivery and adoption | Examples, docs, CI/race/fuzz/integration tests, versions/tags, changelog, Go baseline, downstream adoption, deprecation and removal |

Include only lanes affected by a candidate and show important deferred work explicitly.

## Reference tool flow

Build three grounded views:

1. `Current flow`: host config and admitted resources → package constructor or catalog factory → Eino tool metadata/schema → decoded invocation → validated and bounded execution → stable result → host/runtime event and persistence handling.
2. `Reference lessons`: relevant Pi and OpenCode behavior and trade-offs without copying their TypeScript structures or host-private boundaries.
3. `Candidate delta`: smallest before/after flow, labeling missing, proposed, mocked, external, host-owned, and upstream-owned seams.

For affected flows, test:

- schema and result behavior is deterministic, provider-neutral, bounded, and forward-compatible where promised;
- permissions evaluate final normalized input before side effects, and the leaf exposes enough metadata for the host to decide;
- paths, symlinks, cwd, environment, URLs, subprocesses, and credentials stay within the declared ownership boundary;
- cancellation and deadlines reach blocking I/O and subprocesses, while retry behavior cannot duplicate non-idempotent effects;
- concurrency declarations match actual shared-resource safety, including cross-tool access to one workspace;
- executable-backed tools identify availability and relevant behavior without making volatile machine paths part of stable identity;
- errors remain actionable without leaking secrets or unbounded payloads;
- binary or multimodal results have explicit media detection, size bounds, transport ownership, persistence behavior, and a verified model-visible consumer path;
- credential-free tests exercise the real package and public consumer path; mocks prove only their seam.

## Readiness states

- `Ready`: current repository, dependency, and consumer contracts are sufficient to plan the bounded outcome.
- `Foundation first`: a package or contract foundation must land first or be included.
- `Decision needed`: a material user/product decision changes behavior or architecture.
- `Upstream request likely`: a required sibling contract appears missing; confirm after selection.
- `Blocked upstream`: an unresolved request prevents planning.
- `Discovery`: focused research or contract validation must precede safe planning.
- `Planned`: an existing plan fully claims the outcome; show it as context, not a selectable duplicate.
- `Implemented`: current code and tests already prove the outcome; exclude it unless the candidate adds distinct value.

## Candidate contract

Offer 2–4 candidates, recommendation first. Each selectable option must include:

- concise name;
- type, exactly `add tool`, `strengthen contract`, or `improve delivery`;
- primary consumer and observable agent outcome;
- exact repository evidence or clearly labeled proposed insertion point;
- relevant Pi and OpenCode comparison, preserving differences;
- verified Eino owner, public API/pin, and upstream-request likelihood;
- readiness state;
- before/after tool-flow delta;
- largest dependency or material decision;
- why it forms one coherent implementation plan;
- explicit scope and parity-claim boundary;
- verification approach;
- one-sentence rank rationale.

Candidates must differ in consumer value, capability lane, or dependency trade-off. Do not rank internal refactors, metadata-only additions, docs-only parity tables, or dependency bumps ahead of the nearest repeatable agent journey unless they remove its only proven blocker and have an observable acceptance path.

A working tool must traverse the real metadata/schema, construction, validation, execution, and result path. Consumer-ready claims must also prove mounting and invocation through the supported public `eino-agent` contract. Live credentials may be optional, but deterministic substitutes must still exercise the real public seams.

## Ranking

Use qualitative evidence in this order:

1. Delivers the nearest missing, repeatable coding-agent capability or removes its only hard blocker.
2. Fits public Eino contracts and the correct leaf-versus-host ownership boundary.
3. Exercises a real schema → construction → execution → bounded result → consumer-observable outcome path.
4. Can be verified locally without credentials or uncontrolled effects.
5. Preserves cancellation, bounds, path/process/network safety, deterministic identity, concurrency truth, and compatibility.
6. Establishes a reusable pattern for later tools without prematurely building a broad framework.
7. Avoids parity claims, literal ports, host-policy leakage, and upstream duplication.

Avoid unexplained numeric scores. Preserve a user's different choice and its trade-off.

## Choice and material decisions

Pause after displaying candidates and require explicit selection. Move planned and implemented outcomes outside the list. Label likely upstream work but write requests only after selection and verification.

Ask 1–3 short follow-ups only when answers change public API or observable behavior, such as:

- target `eino-agent` version and consumer acceptance path;
- supported operating systems and required executables;
- leaf-enforced versus host-enforced security or permission behavior;
- text-only results versus typed image/PDF or other attachment transport, including which Eino or host contract owns it;
- in-memory output versus artifact storage for large results;
- compatibility and deprecation policy for schema or identity changes;
- credential-free baseline versus optional configured-service coverage;
- whether a missing upstream capability blocks the milestone or triggers reselection.

Do not silently choose security authority, credential handling, persistence, platforms, or compatibility. An unanswered material decision blocks the brief with owner and exact unblock action.

## Post-selection architecture pass

Answer or explicitly defer: primary consumer and journey; functional and non-functional requirements; platform and resource bounds; ownership and public contracts; current and proposed tool flow; schema/result/identity/concurrency contracts; the component needing a deep dive; cancellation/retry/security/compatibility risks; adoption or migration path; and tests proving the bounded outcome without claiming broad Pi or OpenCode parity.
