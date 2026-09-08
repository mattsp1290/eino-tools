# Upstream Request and Blocking Protocol

Use this reference after selection when a required public contract belongs in another local repository, and on later invocations that must resume an existing blocker.

## Outstanding-request resume gate

Before ordinary frontier discovery:

1. Search `~/.agents/projects/*/requests/` for Markdown fields named `Blocker consumer` whose scalar value is `eino-tools`.
2. Match each request to the same-named file under that project's `responses/` when present.
3. Require exactly one `Request status`: `open`, `withdrawn`, `superseded`, or `resolved`. Missing, repeated, structured, or unknown status blocks until repaired. `resolved` still requires a matching response and verified committed implementation plus a consumable pin; an invalid resolution reopens the blocker.
4. For a declared `Request set`, verify that newly created members agree on consumer, milestone, set ID, and complete member list. A reused open request from an older set is a linked external blocker: list its path in each new member under `Linked reused blockers`, but do not rewrite it or require its older set metadata to match. If an earlier write stopped partway, re-run identity, ownership, deduplication, and safe-path preflight, then create only recorded missing members.
5. If a response declines the ask or changes ownership, report it and ask whether the user wants to withdraw the milestone or supersede it with a named replacement. Stop until that transition is recorded.
6. If requests remain unresolved, list every path, target, selected milestone, set, status, and exact unblock condition, then stop.

After the gate clears, refresh normal repository and external research before making frontier claims.

## Request threshold

Create or reuse one request per distinct target-owned public contract only when:

1. the selected eino-tools acceptance journey requires it;
2. current target code, tests, plans, requests, and responses provide no usable verified public contract and pin;
3. implementing locally would duplicate upstream ownership, require private imports, weaken safety/durability, or create a speculative compatibility surface; and
4. the target and requested contract can be named precisely.

Do not request optional polish, hypothetical comparator parity, broad roadmap work, an already-public API, or behavior eino-tools safely owns.

## Preflight, deduplication, and safe paths

Complete the dependency and ownership map first. Group missing contracts by target; never bundle different owners. Assign new requests one `YYYY-MM-DD-<milestone-slug>` set ID and list every newly created `<target>/<filename>` member. Record equivalent open requests from older sets separately as linked reused blockers; their existing metadata remains unchanged.

Before writing:

1. Resolve the target under `~/git`, verify checkout basename and module/repository identity, read its guidance, and record revision and status without changing it.
2. Inspect relevant public code, tests, accepted plans, tags, requests, and responses.
3. Reuse an equivalent open `eino-tools` request as a linked blocker without changing its milestone or set. Reuse a resolved one only after verifying its response, committed implementation, and pin. Treat another consumer's request as related evidence and create a linked eino-tools blocker.
4. Validate the whole set before creating any file.

Write each request exclusively at `~/.agents/projects/<target>/requests/YYYY-MM-DD-<short-kebab-slug>.md`. Resolve the canonical projects root first. If the verified target project directory or its `requests/` child is absent, create that one exact directory after validating its parent; then resolve and revalidate it before continuing. Require every existing component to be a real, nonsymlink directory, the project and requests directory to be direct children of their parents, and the destination to be a direct child of the requests directory. Reject traversal, nested or absolute slugs, symlink components, and identity mismatches. Never overwrite an existing request.

## Request contents

Include:

```markdown
# Request: <consumer-visible contract>

- **Requested by:** `eino-tools` next-milestone selection
- **Blocker consumer:** `eino-tools`
- **Request status:** `open`
- **Date:** YYYY-MM-DD
- **Priority:** <why it blocks>
- **Selected milestone:** <name>
- **Request set:** <YYYY-MM-DD-milestone-slug>
- **Request set members:** <complete comma-separated target/file list>
- **Linked reused blockers:** <comma-separated existing request paths, or none>
- **Target repo:** <module/repository and checkout>
- **Pinned commit under evaluation:** <full SHA>
- **Consumer:** `eino-tools`

## Background
<Selected journey, repository evidence, owner boundary, and missing seam.>

## Ask
<Smallest acceptable public behavior/contract; label proposed API shapes.>

## Out of scope
<Keep eino-tools implementation, host presentation/policy, unrelated runtime work, and broad parity out of the target.>

## Acceptance
<Public behavior, target tests and gates, docs, compatibility, and a tag or commit eino-tools can pin.>

## Response and unblock contract
- Write the decision or completion record under the target project's `responses/` with this filename.
- The milestone remains blocked until the response identifies a usable contract and its committed implementation and pin are verified.
- If declined, explain the owning boundary or supported alternative so eino-tools can re-scope.

## References
<Exact consumer/target paths and current official sources; no secrets or sensitive payloads.>

## Status history
- YYYY-MM-DD — `open`: created for <milestone>.
```

## Mandatory stop and later transitions

After creating or reusing all blockers, mark the milestone `blocked upstream`; do not create a plan, implementation, private adapter, duplicate, fork, or temporary replacement. Report the milestone, each target, clickable request paths, blocking reason, exact clearing evidence, and whether each request was new or reused, then end the run.

Before changing status/history, repeat the canonical direct-child and no-symlink validation, verify consumer/status/set identity, snapshot current content, and apply only the intended change. Abort on concurrent content or identity drift. Never delete a request or alter another consumer's request.

On explicit withdrawal or supersession, update every open member of this milestone's set consistently and append a dated reason and replacement when applicable. On verified completion, set `resolved` and append the response path, verified commit/tag, and date.

On a later invocation, reopen the request, response, and target repository; verify accepted API, tests, version/tag/commit, and compatibility against committed code. If unresolved, report the same blocker without duplicating it. If consumable, record the pin as `Local dependency fact`, clear the blocker, refresh external research and the repository frontier, and continue.
