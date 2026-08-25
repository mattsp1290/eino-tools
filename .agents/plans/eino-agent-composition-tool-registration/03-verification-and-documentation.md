# Work package 3: verification, documentation, and release contract

## Goal and prerequisite state

After the metadata and catalog contracts compile, prove the requested behavior at leaf, catalog, repository, and consumer-boundary levels. Document the stable identity maintenance process for future changes.

## Exact change surface

### Existing files

- `README.md`: add `catalog` to the package inventory and show metadata enumeration plus workspace materialization without a fake root.
- `CHANGELOG.md`: add the standard catalog, metadata-only accessors, explicit identity policy, option capture, and consumer migration note under `Unreleased`.
- `.github/workflows/ci.yml`: extend dependency hygiene to reject `github.com/mattsp1290/eino-agent` from `catalog` dependencies and direct Go imports, and add a Windows catalog job that runs the non-Unix contract test.

### Proposed files

- `docs/adr/0009-standard-tool-catalog-identity.md` (new, under existing `docs/adr`): record the runtime ownership boundary, explicit IDs, schema-hash preimage, executor-revision policy, binding/concurrency meanings, option capture, consumer identity-composition gate, and rejected registry/generation design.
- `catalog/example_test.go` (new, also listed in work package 2): compile a minimal adapter example.
- `catalog/catalog_unsupported_test.go` (new, non-Unix build constraint, under the new package): assert `Standard` returns `ErrUnsupportedPlatform` and exposes no definitions.

The catalog implementation and tests are specified in [02-standard-catalog.md](02-standard-catalog.md). Leaf-package test changes are specified in [01-leaf-metadata.md](01-leaf-metadata.md).

## Compiling adapter example

The example must:

1. Call `catalog.Standard` with a host-owned option set.
2. Select one returned definition.
3. Call its metadata accessor without a workspace.
4. Map ID, name, schema hash, executor hash, retry safety, and binding kind into a small local `hostRegistration` example type.
5. Wrap `Definition.New` in a host factory that supplies a previously canonicalized workspace root.
6. Keep leaf binding separate from a conceptual global/session mount scope in the example type.
7. Show conceptual versioned composition of the catalog schema hash with host policy identity and of the catalog executor hash with host artifact identity. Do not merely copy the leaf hashes into unused fields.
8. Carry `Concurrent` into the example host registration. For `Concurrent=false`, demonstrate selecting one shared keyed locker for the canonical workspace root, or a registration/dependency key for static bindings, before invoking the factory/tool.
9. Assert in the example translation helper that every safety field is consumed rather than left as unused metadata.
10. State in comments that the real host attaches artifact, config, permissions, retention, and locking policy and mounts the complete translated set atomically.

Do not import `eino-agent` in an example, test, or production file. The example must compile under `go test ./...`.

## Dependency and import guard

Extend the existing CI dependency-hygiene shell block with a targeted check that:

- fails when any Go file under `catalog` imports a path beginning with `github.com/mattsp1290/eino-agent`;
- inspects `go list -deps ./catalog` and fails if that module appears transitively; and
- preserves every existing dependency-hygiene assertion.

Use non-interactive commands and the repository's existing `rg` and `go list` patterns.

## Verification matrix

| Layer | Required proof | Command or procedure |
|---|---|---|
| Leaf metadata | Builders match existing receiver metadata and exported schema | `go test ./fileops ./glob ./search ./applypatch ./shell ./urlfetch ./userinteract ./trackerwrite` |
| Catalog contract | Stable ordering, unique identity, metadata-only access, options, errors, and fresh factories | `go test ./catalog` |
| Root binding and isolation | Every workspace definition observes its supplied root; mutations cannot cross roots | Table-driven `catalog` integration-style unit tests using two `t.TempDir()` roots |
| Concurrency declarations | Every standard ID has the exact table tuple | Golden table test in `catalog/catalog_test.go` |
| Identity stability | Schema and executor hashes match intentional golden values | Golden table test in `catalog/catalog_test.go` |
| External provenance | PATH/environment changes cannot redirect frozen search/shell; executable replacement is rejected | Focused non-parallel `catalog` tests with temporary executables and restored globals |
| Race safety | Tests do not reveal shared catalog/container races | `go test -race ./...` |
| Dependency boundary | Catalog cannot import consumer runtime | CI hygiene block and local equivalent |
| Platform contract | Public catalog builds on Windows and returns the documented unsupported error | Windows CI job: `go test ./catalog` |
| Module consistency | No dependency drift | `go mod tidy -diff` |
| Repository tests | All packages pass | `make test` |
| Static analysis | Vet and configured linters pass | `make vet` and `make lint` |

## Acceptance criteria

1. Every acceptance item in `.agents/requests/eino-agent-composition-tool-registration/03-acceptance.md` maps to at least one automated test or compiling example.
2. The README and ADR describe `Concurrent=false` as a host locking requirement, not an internal lock.
3. The ADR tells future maintainers exactly when to update schema golden values and when to increment executor revisions.
4. The changelog states that the external consumer must translate the catalog into its composition registry and can delete the disconnected helper after adoption.
5. The ADR and example distinguish leaf binding from global/session mount scope and warn that static URL fetch still requires host permission policy.
6. The ADR records the consumer identity gate: leaf schema and executor hashes must affect the final persisted `session.ToolPlanIdentity` rather than being discarded.
7. The ADR assigns opaque injected policy to host configuration identity and records how shell/search executable and environment fingerprints enter leaf executor identity.
8. The adapter example consumes `Concurrent` and distinguishes workspace-root locking from static shared-dependency locking.
9. The README states that the full standard catalog executes on Unix and that non-Unix `Standard` returns `ErrUnsupportedPlatform`.
10. No documentation claims that `eino-agent` adoption, run-plan persistence, or helper deletion occurred in this repository.
11. All commands in the verification matrix pass from the repository root.

## Risks and edge cases

- Golden hashes can become busywork if tests do not explain the semantic reason for each update. Failure output must identify the registration ID and old/new value.
- Schema comparison must use parsed JSON or `ToJSONSchema`; raw whitespace changes in existing schema literals must not create accidental semantic-test failures. The published hash intentionally follows the canonical marshaled form, not source formatting.
- The local consumer checkout is evidence for the adapter boundary, not an implementation dependency or a test fixture.
- Environment and executable tests mutate process-global state. They must not use `t.Parallel`, must restore values with cleanup, and must use private temporary paths.
- Lint invokes a pinned toolchain and can take longer than unit tests. Run it after focused tests pass.

## Exclusions

- Do not modify the sibling `eino-agent` checkout.
- Do not add cross-repository tests.
- Do not publish a module version or change `go.mod` unless implementation introduces an actually required dependency. The proposed design requires none.
