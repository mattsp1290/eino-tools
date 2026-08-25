# ADR 0009: Standard tool catalog identity and ownership

Status: Accepted

## Context

Consumers need a deterministic description of the reusable `eino-tools` leaf
set before they bind a concrete workspace. Constructing tools against `/` to
inspect metadata conflates description with authority, and a mutable leaf-side
registry cannot represent a host's run-plan, persistence, permission, or mount
lifecycle.

## Decision

The `catalog` package exposes an ordered standard bundle of immutable-by-
convention `Definition` values. Registration IDs are explicit authored
constants and remain separate from model-visible names. A metadata accessor
returns a fresh `ToolInfo` without constructing a tool. Each factory returns a
fresh leaf object from options captured when `Standard` is called.

`BindingWorkspace` means the factory requires an already admitted canonical
workspace root. `BindingStatic` means no root is needed. Binding does not choose
global or session mount scope, component lifetime, retention, or permissions.
In particular, static URL fetch still needs host filesystem and network policy.

`Concurrent=false` is a host locking requirement, not an internal catalog lock.
The host uses one shared keyed lock across all workspace definitions for a
canonical root, and a registration/dependency key for a non-concurrent static
definition. Independent canonical roots may execute concurrently.

The complete Unix catalog is validated before return. Invalid options,
definitions, metadata, or constructors produce errors rather than a partial
bundle. Non-Unix builds retain the public types and return
`ErrUnsupportedPlatform` with no definitions.

## Identity records

Schema identity is lowercase SHA-256 of canonical JSON with fixed fields:

```text
version = "eino-tools-tool-schema-v1"
name
description
parameters = ToolInfo.ParamsOneOf.ToJSONSchema()
```

Update a schema golden whenever the model-visible name, description, or
parameter schema intentionally changes. Source formatting that leaves the
canonical schema unchanged does not change identity.

Executor identity is lowercase SHA-256 of canonical JSON with fixed fields:

```text
version = "eino-tools-tool-executor-v1"
registration_id
revision
dependencies = ordered kind/path/content-sha256 records
environment = captured-environment sha256 or empty
```

Every explicit ID has its own positive executor revision. Increment only the
affected revision whenever execution semantics can change a resumed call,
including constructor defaults, output/error behavior, dependency invocation,
path handling, timeouts, concurrency requirements, or option interpretation. A
schema-only change does not require an executor bump. Changing an identity
record version or hash algorithm is itself an identity break.

For search and shell, `Standard` normalizes and snapshots the environment,
resolves an absolute invocation path through that captured `PATH`, preserves
that path so symlink aliases retain their `argv[0]` semantics, resolves and
hashes the executable target, and includes both paths plus the environment
digest in executor identity. Materialization rejects a changed target or
changed executable bytes. Environment and executable paths must be valid UTF-8
so JSON identity records cannot collapse distinct Unix byte strings. The
deployment must keep the verified invocation and target paths immutable after
materialization or enforce an equivalent invocation-time artifact guarantee.

Opaque injected dependencies—HTTP policy, user-interaction I/O, tracker writer
configuration, and deployment immutability guarantees—have no canonical Go
value hash. They belong in host configuration identity.

## Consumer gate

Catalog hashes are leaf-only identity inputs. A consumer must compose the leaf
schema hash with host permission, retry, retention, and metadata identity, and
compose the leaf executor hash with host artifact/executor identity. Both
composed values must reach the final persisted run-plan identity and participate
in resume drift checks. Discarding either catalog hash, including overwriting
executor identity with only a host artifact hash, blocks adoption.

The host also owns workspace admission and lifetime, permissions, retention,
artifact/config identity, global/session mount scope, atomic mounting, run-plan
persistence, and shared serialization. This repository does not implement or
claim `eino-agent` adoption.

## Rejected alternatives

- A leaf-side mutable register/replace/unregister registry or generation API:
  the host already owns mount lifetime and frozen run plans.
- Fake-root construction for metadata: it grants irrelevant authority and can
  apply mutable defaults while merely inspecting a schema.
- Derived registration IDs: explicit IDs avoid accidental identity changes when
  model names change.
- A catalog-owned keyed locker: workspace authority and cross-tool
  serialization belong to the host.
