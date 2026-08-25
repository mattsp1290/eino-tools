# Execution handoff

## Planning state

Status: Ready. Implementation has not occurred.

Application context, compatibility policy, and architectural decisions are authoritative in [00-overview.md](00-overview.md).

## Before implementation

1. Run `bd ready` and locate the implementation bead created from this plan, or create one if it does not exist.
2. Inspect `git status --short --branch` and preserve unrelated user changes.
3. Claim the implementation bead with `bd update <id> --claim`.
4. Run `make test` to establish the baseline.
5. Stop if the baseline fails for a reason unrelated to this change; record the failure on the bead before modifying code.

## Dependency-ordered work packages

### WP1: Extract metadata-only builders

Result: all eleven leaf definitions expose `ToolInfo` without a tool instance.

- Follow [01-leaf-metadata.md](01-leaf-metadata.md).
- Change existing symbols at the documented insertion points in `fileops`, `glob`, `search`, `applypatch`, `shell`, `urlfetch`, `userinteract`, and `trackerwrite`.
- Add the proposed package-level metadata symbols and delegate existing receiver methods.
- Run the focused leaf-package test command from the verification matrix.
- Acceptance gate: package-level and receiver metadata are semantically identical, metadata calls require no workspace, and no schema or description changes.

Prerequisites: clean baseline. Parallelization: leaf packages may be changed independently, but complete and integrate all metadata builders before WP2.

### WP2: Pin external executors and add catalog identity primitives

Result: the new `catalog` package can describe and validate one definition with deterministic schema and executor identity.

- Add proposed `search/options.go`; update existing `search/search.go` and `search/search_test.go` so catalog callers can inject an absolute `rg` path and captured environment while direct defaults remain available.
- Create proposed `catalog/doc.go`, `catalog/types.go`, `catalog/catalog_unix.go`, `catalog/catalog_unsupported.go`, and `catalog/identity.go` under the existing repository root.
- Add the public types, binding constants, explicit ID constants, versioned JSON hash records, executor revisions, and private validation.
- Add focused identity, external-provenance, and invalid-definition tests in proposed `catalog/catalog_test.go`.
- Acceptance gate: metadata hashing is deterministic, IDs are literals, `rg` and shell resolution cannot drift through PATH/environment changes, invalid records return errors, and the package has no consumer import.

Prerequisites: WP1. Parallelization: identity helpers and public type scaffolding may proceed together after metadata symbol names are fixed.

### WP3: Assemble options and factories

Result: `catalog.Standard` returns the fixed 10- or 11-entry validated bundle and each entry materializes a fresh correctly bound leaf tool.

- Follow the option capture, default, ordering, binding, safety, factory, and atomicity contracts in [02-standard-catalog.md](02-standard-catalog.md).
- Add deterministic enumeration, option snapshot, typed-nil rejection, cancellation, uniform canonical-root validation, all-definition fresh-instance, optional tracker, and per-workspace-tool two-root behavior tests.
- Acceptance gate: all catalog tests pass and the returned definitions exactly match the overview table.

Prerequisites: WP2. Parallelization: factory tests can be divided by workspace and static definitions, but bundle ordering and validation have one integration owner.

### WP4: Document and enforce the boundary

Result: consumers and future maintainers can adopt the catalog and maintain identities without reconstructing design intent.

- Follow [03-verification-and-documentation.md](03-verification-and-documentation.md).
- Add proposed `docs/adr/0009-standard-tool-catalog-identity.md` under existing `docs/adr`.
- Add proposed `catalog/example_test.go` under the new package.
- Add proposed `catalog/catalog_unsupported_test.go` under the new package and a Windows catalog CI job.
- Update existing `README.md`, `CHANGELOG.md`, and `.github/workflows/ci.yml`.
- Acceptance gate: the adapter example compiles, CI rejects consumer imports, and documentation does not claim external implementation.

Prerequisites: WP3 public API is stable. Parallelization: ADR, README, and changelog can proceed in parallel after API names settle; CI and example tests must use the final package shape.

### WP5: Integrate and close

Result: the repository is green, the implementation bead is complete, and all changes are pushed.

Run in this order:

1. `gofmt` and `goimports` on changed Go files.
2. `go test ./fileops ./glob ./search ./applypatch ./shell ./urlfetch ./userinteract ./trackerwrite ./catalog`
3. `go mod tidy -diff`
4. `make test`
5. `make vet`
6. `go test -race ./...`
7. `make lint`
8. Cross-compile the catalog package for Windows with `catalog_cross_dir=$(mktemp -d)`, `GOOS=windows GOARCH=amd64 go test -c ./catalog -o "$catalog_cross_dir/catalog.test.exe"`, and `rm -rf -- "$catalog_cross_dir"` after a successful compile.
9. Run the catalog dependency-hygiene commands locally.

If a hash golden changes, inspect the canonical preimage and confirm the schema input or executor revision changed intentionally before accepting it.

Prerequisites: WP1 through WP4. Parallelization: none for the final gate.

## Integration and regression gates

- No package imports `github.com/mattsp1290/eino-agent`.
- The default catalog has exactly 10 entries; configuring tracker write produces exactly 11.
- The catalog order, IDs, names, binding kinds, safety declarations, and identity hashes are deterministic.
- Existing leaf constructors and receiver `Info` methods preserve behavior.
- Metadata access never requires a workspace.
- Workspace factories never use cwd or `/` as a sentinel. They uniformly reject missing, non-directory, non-canonical, and removed roots while leaving authorization/admission to the host.
- Search and shell use captured absolute executables and environments. Materialization rejects a changed executable fingerprint.
- Host options are captured once and do not change when caller-owned option containers are later reassigned or mutated.
- Factory errors expose no tool, and catalog validation errors expose no partial slice.
- Documentation assigns composition, permissions, retention, artifact/config identity, locking, mount atomicity, and run-plan persistence to the consumer.

## Final definition of done

1. Every requested standard tool appears under the documented condition.
2. Every requested identity, lifecycle, error, isolation, and ownership invariant has automated or compiling-example coverage.
3. All verification commands pass.
4. The implementation bead records any intentional schema or executor identity changes and is closed.
5. Any external `eino-agent` adoption work remains a separately tracked consumer task.
6. Repository changes are committed, `git pull --rebase` succeeds, `bd dolt push` succeeds, `git push` succeeds, and `git status` reports the branch up to date with origin.

## Deferred work

- Translate catalog definitions into `eino-agent/composition.ToolRegistration` records in the consumer repository.
- Extend consumer identity composition so the leaf schema hash contributes to persisted schema identity and the leaf executor hash combines with host artifact identity in persisted executor identity.
- Add consumer tests that changing either leaf identity changes `session.ToolPlanIdentity` and causes resume drift rejection.
- Add a consumer rollback test for a descriptor persisted before version rollback. Document that post-adoption runs require matching artifacts or explicit abandonment/recreation.
- Delete `eino-agent/tools/einotools.RegisterDefaults` and its private registry path after the composition adapter is covered.
- Decide consumer-owned permission, retention, artifact, config, instance, order, and mount values.

These items are out of scope for this repository and must not block this plan's implementation.
