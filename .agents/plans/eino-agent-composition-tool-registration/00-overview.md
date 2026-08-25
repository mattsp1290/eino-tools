# Eino-agent composition tool registration plan

Status: Ready

Planning only. No implementation described by this plan has occurred.

## Application context

```json
{
  "application_context": {
    "has_active_users": false,
    "backward_compatibility_required": false,
    "feature_flags": "not-applicable",
    "confirmation_digest": "2443775a1cf84c80027f30cac6dd425a5591aa0f746db2e07e2cb837d31ca084",
    "confirmed_at": "2026-08-25T02:39:30Z"
  }
}
```

The user confirmed that `eino-tools` has no active users or external consumers and that this change does not need backward compatibility. The implementation can introduce the correct provider boundary directly. It does not need feature flags, aliases, migrations, or a compatibility wrapper for `eino-agent/tools/einotools.RegisterDefaults`.

## Change classification

- Change type: additive public Go API, internal metadata refactor, test expansion, and package documentation.
- Affected areas: the eight leaf-tool packages, a new injectable search execution option, a new standard catalog package, dependency-hygiene CI, README, changelog, and a new architecture decision record.
- External consumer boundary: `github.com/mattsp1290/eino-agent` translates catalog entries into its own `composition.ToolRegistration` records. This repository must not import that module.

## Requested outcome

Expose a deterministic, runtime-neutral catalog for every standard `eino-tools` leaf tool. Each catalog definition separates stable registration identity from the model-visible name, exposes metadata without constructing a workspace tool, declares binding and execution safety, publishes restart-stable schema and executor hashes, and materializes a fresh Eino `tool.InvokableTool` from captured options and a host-supplied workspace instance.

## Success criteria

1. Two calls to the standard catalog return the same ordered IDs, names, binding kinds, retry declarations, concurrency declarations, schema hashes, and executor hashes for the same options and external-dependency snapshot.
2. The default catalog contains file read, write, edit, and list plus glob, search, apply-patch, shell, URL fetch, and user interaction. It adds tracker write only when a non-nil tracker writer is configured.
3. Metadata inspection succeeds without constructing a tool, supplying a workspace root, recapturing process cwd, or using `/` as a sentinel.
4. Every workspace factory rejects a root that is missing, not a directory, or not already canonical. Two materializations receive the exact admitted host roots and return distinct tools whose later behavior is isolated by root.
5. Invalid configured options, invalid definitions, invalid workspace instances, and leaf constructor failures return errors. They do not silently omit configured tools or expose a partial catalog.
6. The catalog package has no dependency on `github.com/mattsp1290/eino-agent` and does not implement run plans, permissions, persistence, or mount lifetime.
7. Package documentation contains a compiling adapter example that maps one catalog definition into a host-owned registration shape.
8. `make test`, `make vet`, `make lint`, `go test -race ./...`, and `go mod tidy -diff` pass.

## Scope

- Extract instance-independent `ToolInfo` builders from the existing leaf tool methods without changing model-visible metadata.
- Add the proposed `catalog` package at the existing repository root.
- Define a standard bundle with explicit IDs, deterministic ordering, binding kind, retry safety, concurrency policy, identity hashes, metadata accessors, and factories.
- Capture shell, URL-fetch, user-interaction, and tracker dependencies in catalog options.
- Make search's `rg` executable and environment injectable so the catalog can pin them instead of resolving mutable process state during calls.
- Validate the complete catalog before returning it.
- Add contract, isolation, error, import-boundary, and identity regression tests.
- Document the adapter boundary and the maintenance rule for executor revisions.

## Non-goals

- Implement `eino-agent` composition, permissions, retention, configuration identity, artifact identity, session scope, workspace admission, run-plan persistence, or resume drift checks.
- Preserve or replace the disconnected `eino-agent/tools/einotools.RegisterDefaults` helper in this repository.
- Add registration generations, dynamic replace/unregister behavior, feature flags, migration code, aliases, or deprecation shims.
- Move plan, retained-output, subagent, skill-loading, web-search, or LSP capabilities into `eino-tools`.
- Add a package-level keyed locker. The host owns workspace authority and shared serialization.
- Change any leaf schema, description, result envelope, or execution behavior as part of metadata extraction.

## Repository findings

### Verified facts

- `go.mod` declares module `github.com/mattsp1290/eino-tools`, Go 1.26, and Eino v0.8.13.
- `fileops`, `glob`, `search`, `applypatch`, `shell`, `urlfetch`, `userinteract`, and `trackerwrite` already implement Eino `tool.InvokableTool` and export deterministic schema bytes. `fileops` exports four schema functions; each other leaf package exports `Schema`.
- Every existing `Info(context.Context)` method embeds its description in the concrete tool method. Workspace tools therefore have no supported metadata-only API even though their metadata does not read receiver state.
- Workspace constructors require an absolute root. `glob`, `search`, and `applypatch` resolve symlinks at construction; `fileops` validates and canonicalizes; `shell` records the supplied absolute root.
- `shell.Options`, `urlfetch.Options`, `userinteract.Options`, and `tracker.CloseWriter` are the current host-injectable execution surfaces.
- `search` currently hard-codes `rg` and inherits process environment at invocation. `shell` resolves a relative/default `sh` and a nil environment from mutable process state. Those behaviors are not restart-stable executor provenance for a frozen catalog.
- `docs/adr/0008-workspace-filesystem-serialization.md` requires one shared per-canonical-root serialization boundary for `fileops`, `glob`, `search`, and `apply_patch`. The existing `eino-agent/tools/einotools` adapter also serializes shell because its workspace specifications all default `concurrent` to false.
- `userinteract` defaults are environment-backed and its MCP surface assumes one outstanding question. `tracker.CloseWriter` does not promise concurrency safety.
- The existing consumer adapter creates metadata by constructing workspace tools with `/`, registers them in a mutable private `tools.Registry`, and materializes tools by consulting runtime workspace data. The request explicitly replaces this disconnected boundary.
- `composition.Registry` in the local `eino-agent` checkout owns mount atomicity, global/session mount scope, artifact/config provenance, immutable run plans, and descriptor persistence. Those remain consumer responsibilities.
- The current consumer has no catalog executor-hash field. `composition.Registrar.Tool` replaces `tools.Definition.Provenance.ExecutorHash` with the component artifact hash, while its persisted schema hash is recomputed from model metadata plus host permissions, retry safety, retention, and metadata. Consumer adoption therefore needs an explicit identity-composition change; copying catalog hashes into an adapter-local record is insufficient.
- CI already runs module tidiness, vet, lint, race tests, and dependency hygiene. It does not currently prohibit an `eino-agent` import in a new catalog package.
- The repository is tagged `v0.1.0`, and `CHANGELOG.md` states that pre-v1 minor releases may break compatibility when migration notes are present.
- `shell/shell.go` is Unix-only. An untagged catalog that directly calls `shell.New` would not compile on non-Unix systems even though current CI runs only Ubuntu.

### Assumptions

- A host interprets `RetrySafe` as safe to repeat after an uncertain completion because the operation has no intended external mutation. It does not mean the repeated result is guaranteed byte-for-byte identical when underlying data changes.
- A host interprets `Concurrent` as permission to execute calls concurrently against the same canonical workspace or shared static dependency. `false` requires host-side serialization; the catalog does not supply the lock.
- Standard-library `http.Client` and its transport contract permit concurrent use. A host that injects a policy client must honor that contract.
- The consumer will attach its own permission, retention, artifact, config, instance, and mount identity. The catalog hashes identify only leaf schema and executor semantics.

## Key decisions

1. Add the proposed `catalog` package and proposed `Standard(Options) ([]Definition, error)` entry point. The package is runtime-neutral and depends only on Eino and the leaf packages in this module.
2. Add proposed package-level metadata builders to each leaf package and make existing tool methods delegate to them. The catalog never creates a fake workspace tool to obtain metadata.
3. Use explicit, exported registration-ID constants. Do not derive an ID from the tool name or concatenate identity fields.
4. Compute schema and executor identities from versioned JSON records and SHA-256. Schema records contain model-visible metadata. Executor records contain the explicit registration ID and a manually maintained executor revision.
5. Return fresh metadata and tool instances. Snapshot mutable option containers when `Standard` runs, validate the entire definition slice before returning it, and omit only tracker write when its explicit dependency is absent.
6. Resolve and fingerprint external executables when the Unix catalog is created, capture execution environments, and verify executable fingerprints again at materialization. Treat opaque injected dependency identity as consumer-owned configuration identity.
7. Validate workspace instance structure uniformly in the catalog. This confirms usability at factory return time without taking workspace authorization away from the host.
8. Provide a non-Unix `Standard` stub that returns a documented unsupported-platform error while keeping the package's public types buildable.

## Standard bundle policy

| Order | Explicit registration ID | Model name | Binding | RetrySafe | Concurrent |
|---:|---|---|---|---:|---:|
| 1 | `standard.file-read` | `file_read` | workspace | true | false |
| 2 | `standard.file-write` | `file_write` | workspace | false | false |
| 3 | `standard.file-edit` | `file_edit` | workspace | false | false |
| 4 | `standard.file-list` | `file_list` | workspace | true | false |
| 5 | `standard.glob` | `glob` | workspace | true | false |
| 6 | `standard.search` | `search` | workspace | true | false |
| 7 | `standard.apply-patch` | `apply_patch` | workspace | false | false |
| 8 | `standard.shell` | `shell` | workspace | false | false |
| 9 | `standard.url-fetch` | `url_fetch` | static | true | true |
| 10 | `standard.user-interact` | `user_interact` | static | false | false |
| 11 | `standard.tracker-write` | `tracker_write` | static | false | false |

Tracker write occupies the final position only when `Options.TrackerWriter` is non-nil. Every workspace definition is non-concurrent so the host can place file discovery, mutation, search, and shell execution behind one keyed locker for the canonical root. Independent roots may run concurrently.

`Binding` describes whether a leaf factory needs a canonical workspace root. It is not the consumer's global/session mount scope, permission scope, or component lifetime. A static binding can still be mounted globally or per session, can require permissions, and can access external resources. In particular, `url_fetch` can read absolute `file://` paths and HTTPS endpoints.

## Target flow

```text
leaf package ToolInfo builder
          |
          v
catalog.Standard(captured options)
  -> validate every definition
  -> return ordered immutable-by-convention descriptors
          |
          v
consumer adapter (eino-agent-owned)
  -> attach artifact/config/permission/retention/mount identity
  -> keep leaf binding separate from global/session mount scope
  -> compose catalog hashes with host-owned persisted identities
  -> mount composition.ToolRegistration records atomically
          |
          v
frozen run plan calls Definition.New(ctx, Instance{WorkspaceRoot: canonicalRoot})
  -> fresh Eino InvokableTool or error
```

## Consumer identity handoff

The catalog publishes leaf-only identity. A later `eino-agent` change must carry both catalog hashes through translation and trace them to the final `session.ToolPlanIdentity`:

- Persisted schema identity must be a versioned hash over `Definition.SchemaHash` plus host-owned permissions, retry safety, retention, and metadata. The host must not silently discard the leaf schema hash or rely on an undocumented equivalent recomputation.
- Persisted executor identity must be a versioned hash over `Definition.ExecutorHash` plus the host artifact/executor identity. The current behavior that overwrites executor identity with only `component.Artifact.Hash` must change before adoption.
- Consumer tests must prove that changing only the leaf schema hash changes the persisted schema identity, changing only the leaf executor hash changes the persisted executor identity, and resuming with either changed identity is rejected.
- Host configuration identity must cover opaque injected policies and mutable deployment inputs that the catalog cannot inspect, including URL-fetch client policy, user-interaction surface/I/O policy, tracker writer configuration, and any deployment guarantee that keeps a verified executable path immutable after materialization.

This repository documents and examples the inputs but does not modify `eino-agent`. The external identity-composition change is a mandatory consumer-adoption gate, not a prerequisite for implementing the catalog itself.

## Compatibility, rollout, migration, and rollback

- Compatibility: not required by user decision. Do not add a compatibility layer in this module.
- Rollout: publish the catalog as a normal library change. The external consumer can adopt it only after its persisted schema/executor identity composition carries the catalog hashes through to `session.ToolPlanIdentity`; it can then delete its disconnected helper.
- Stored data and configuration: this repository owns neither. No migration applies here.
- Established workflows: none require preservation. The quality-gate commands remain unchanged.
- Rollback before persistence: before any catalog-backed descriptor is stored, revert the catalog commit or pin the consumer to the preceding `eino-tools` version.
- Rollback after persistence: retain or redeploy the exact matching consumer, catalog, and external-executable artifacts until affected runs finish. If that is impossible, explicitly abandon and recreate those runs. Rolling both versions back does not make newer persisted identities resumable by older code.

## Risks and gates

- Stop if an implementation needs an `eino-agent` import. Move that translation to the consumer.
- Stop consumer adoption if either catalog hash is discarded before `session.ToolPlanIdentity` persistence.
- Stop if metadata extraction changes any name, description, or JSON schema. That is a separate model-visible change and requires explicit schema-hash review.
- Stop if executor behavior changes without an executor-revision bump and updated golden identity tests.
- Stop if shell or search would resolve a relative executable or inherit a later process environment during invocation.
- Stop if a host cannot keep a fingerprint-verified executable path immutable for the lifetime of a frozen run plan; that deployment needs invocation-time verification or a stronger sandbox artifact contract.
- Treat a changed identity hash as expected only when its documented input changed. Investigate nondeterministic hash changes before merge.
- Do not mark the catalog concurrent merely because an individual Go object has no mutable fields. The declaration covers shared workspace or injected-dependency safety.

## Decisions and open questions

- Blocking decisions: none.
- Non-blocking decisions: none. The implementation can choose private helper names, but it must preserve the proposed public API and observable contracts in this plan.

## Document map

- [01-leaf-metadata.md](01-leaf-metadata.md) extracts metadata without constructing tools.
- [02-standard-catalog.md](02-standard-catalog.md) defines identities, options, validation, ordering, and factories.
- [03-verification-and-documentation.md](03-verification-and-documentation.md) specifies tests, CI guards, examples, and release documentation.
- [04-execution-handoff.md](04-execution-handoff.md) orders implementation work and defines merge gates.
