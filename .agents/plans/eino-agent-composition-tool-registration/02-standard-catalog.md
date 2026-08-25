# Work package 2: deterministic standard catalog

## Goal and prerequisite state

After [01-leaf-metadata.md](01-leaf-metadata.md) lands, add a runtime-neutral standard catalog that returns validated definitions in a fixed order and materializes fresh Eino leaf tools from captured configuration.

## Proposed files

All files below are new and use existing parent paths:

- `catalog/doc.go` (new): package contract, binding/concurrency semantics, identity maintenance rules, and adapter overview.
- `catalog/types.go` (new, under the repository root): portable public types, ID constants, options, and unsupported-platform error.
- `catalog/catalog_unix.go` (new, under the repository root): Unix standard-bundle assembly, validation, executable provenance, and factories.
- `catalog/catalog_unsupported.go` (new, under the repository root): non-Unix `Standard` implementation that returns the unsupported-platform error.
- `catalog/identity.go` (new): versioned canonical identity records and SHA-256 helpers.
- `catalog/catalog_test.go` (new, Unix build constraint): definition, identity, option, factory, isolation, and failure tests.
- `catalog/catalog_unsupported_test.go` (new, non-Unix build constraint): `Standard` unsupported-platform behavior.
- `catalog/example_test.go` (new, Unix build constraint): compiling host-adapter example without an `eino-agent` dependency.
- `search/options.go` (new, under existing `search`): injectable `rg` executable and environment values.

Modify existing `search/search.go` so `New` accepts at most one proposed `search.Options`, snapshots its environment, records an absolute `rg` path when supplied, passes the captured environment to `exec.Cmd`, and conditionally disables user ripgrep config with `--no-config`. Modify existing `search/search_test.go` for default preservation and injected dependency tests.

The proposed search option shape is:

```go
type Options struct {
    RGBinary     string
    Env          []string
    DisableConfig bool
}
```

An empty `RGBinary` preserves the direct leaf default `rg`. A nil `Env` preserves direct leaf environment inheritance. `DisableConfig=false` preserves the current direct-leaf behavior. The catalog never passes either default through unresolved and always sets `DisableConfig=true`.

Private catalog helpers may move between the proposed catalog files during implementation. Any additional non-Unix file must stay under `catalog`; any search option code must stay under `search`.

## Proposed public contract

The exact comments and private helper names are implementation details. Preserve this public shape unless repository compilation reveals a necessary type adjustment:

```go
type BindingKind string

const (
    BindingStatic    BindingKind = "static"
    BindingWorkspace BindingKind = "workspace"
)

type Instance struct {
    WorkspaceRoot string
}

type Definition struct {
    ID           string
    Name         string
    Binding      BindingKind
    RetrySafe    bool
    Concurrent   bool
    SchemaHash   string
    ExecutorHash string
    Info         func() (*schema.ToolInfo, error)
    New          func(context.Context, Instance) (tool.InvokableTool, error)
}

type Options struct {
    SearchOptions   *search.Options
    ShellOptions    *shell.Options
    URLFetchOptions *urlfetch.Options
    UserSurface     userinteract.Surface
    UserOptions     userinteract.Options
    TrackerWriter   tracker.CloseWriter
}

func Standard(options Options) ([]Definition, error)
```

Add proposed exported constants `IDFileRead`, `IDFileWrite`, `IDFileEdit`, `IDFileList`, `IDGlob`, `IDSearch`, `IDApplyPatch`, `IDShell`, `IDURLFetch`, `IDUserInteract`, and `IDTrackerWrite` with the literal values in the overview table. The values are authored constants. They are not derived from names or assembled with delimiters.

Add proposed exported sentinel `ErrUnsupportedPlatform`. The non-Unix `Standard` implementation returns an error wrapping this sentinel and a nil definition slice.

`BindingKind` describes factory input only. It must not be named or documented as global/session mount scope, runtime permission scope, or component lifetime. The consumer chooses those host-owned dimensions independently.

## Option capture and defaults

1. An empty `Options` value includes every standard tool except tracker write.
2. Empty `UserSurface` resolves to `userinteract.SurfaceMCP`, matching the existing consumer adapter's non-blocking default.
3. A non-empty user surface must be `SurfaceCLI` or `SurfaceMCP`; otherwise `Standard` returns an error and no definitions.
4. A nil `ShellOptions` or `URLFetchOptions` selects the leaf constructor defaults.
5. A nil `SearchOptions` selects `rg` from the captured process environment at `Standard` time. Copy a supplied value and clone its environment slice.
6. Snapshot `os.Environ()` at `Standard` time when search or shell receives a nil environment. Do not leave nil in the captured leaf options.
7. Normalize each captured environment to deterministic execution semantics: reject invalid entries and NUL bytes, apply last-value-wins for duplicate keys, sort by key, and use the normalized slice both for execution and hashing.
8. Resolve a bare `rg` or shell executable name against the normalized captured `PATH`, not later process state. Accept an already absolute path. Reject relative paths containing separators so resolution never consults cwd. Reject missing, non-regular, or unreadable executables.
9. Hash the bytes of each resolved external executable and hash the normalized captured environment without exposing environment values. Store these digests in the affected executor identity preimages.
10. If `ShellOptions` is non-nil, copy the value and clone its `Env` slice. Reject a negative output cap before returning definitions.
11. If `URLFetchOptions` is non-nil, copy the value when `Standard` runs. Retain the supplied `HTTPClient` pointer as the explicit shared policy dependency.
12. Copy `UserOptions` when `Standard` runs. Resolve nil `Stdin` and `Stderr` to the then-current `os.Stdin` and `os.Stderr` so later reassignment of those package variables cannot change factory configuration. The MCP surface still passes no reader into the constructed leaf tool.
13. For CLI user interaction, reject a non-nil interface whose dynamic value is nil for either `Stdin` or `Stderr`. For MCP, ignore these unused fields after capture.
14. Capture `TrackerWriter` once. Omit only `standard.tracker-write` when this interface is nil. Reject a non-nil interface whose dynamic value is nil; do not treat it as optional absence.
15. Use a private reflection-based nil-like check limited to interface kinds that can be nil. Never call `IsNil` for a non-nilable kind.
16. Every factory call invokes the leaf constructor and returns a new concrete tool. Shared injected dependencies such as an HTTP client or tracker writer may be reused by design.

## Definition assembly and validation

Build the complete slice locally in the order declared in `00-overview.md`. Before returning it:

1. Require a known binding kind, non-empty explicit ID, non-empty name, non-empty lowercase 64-character hexadecimal hashes, non-nil metadata accessor, and non-nil factory.
2. Reject duplicate IDs and duplicate model-visible names.
3. Call each metadata accessor without an `Instance`. Require the returned name to equal `Definition.Name` and require a valid `ParamsOneOf` schema.
4. Recompute `SchemaHash` from the returned metadata and require equality with the stored field.
5. Return `nil, error` for any definition or option error. Do not return the valid prefix.

Keep validation private unless a consumer requirement independently justifies a public validator.

## Identity contract

### Schema hash

Compute `SchemaHash` as lowercase SHA-256 hex over `encoding/json.Marshal` of this proposed fixed-field record:

```text
version:     "eino-tools-tool-schema-v1"
name:        Definition.Name
description: ToolInfo.Desc
parameters:  ToolInfo.ParamsOneOf.ToJSONSchema()
```

Use a Go struct, not a map, for the outer record. `encoding/json` provides deterministic struct-field order and sorts string map keys inside the parameter schema. A change to model-visible name, description, or parameter schema must change this hash. Do not include host permissions, retention, metadata, artifact, configuration, or mount scope; the consumer owns those identity dimensions. Consumer adoption must combine this leaf hash with its host-owned schema dimensions as specified in `00-overview.md`.

### Executor hash

Compute `ExecutorHash` as lowercase SHA-256 hex over `encoding/json.Marshal` of this proposed fixed-field record:

```text
version:         "eino-tools-tool-executor-v1"
registration_id: Definition.ID
revision:        <positive integer maintained for that explicit ID>
dependencies:    <ordered kind/path/content-sha256 records, or empty>
environment:     <sha256 of captured environment for environment-sensitive executors, or empty>
```

Maintain one private positive executor revision beside each explicit ID. Initialize every new definition at revision 1. Increment only the affected revision whenever execution semantics could change a resumed call, including constructor defaults, output/error behavior, dependency invocation, path handling, timeouts, concurrency requirements, or injected-option interpretation. A schema-only description change does not require an executor bump unless execution also changes. Sort dependency records by kind and path before marshaling.

The JSON record makes identity boundaries unambiguous and avoids magic-delimiter concatenation. Document that changing the hash algorithm version is itself an identity break.

For shell and search, the leaf executor hash includes the resolved executable path, executable-content digest, and captured-environment digest. At materialization, recompute the executable digest and return an identity-drift error before construction if it changed. The deployment must keep the verified path immutable after materialization or verify again at invocation.

Opaque injected behavior remains consumer configuration identity because Go interfaces such as `http.Client` and `tracker.CloseWriter` do not have a canonical semantic hash. Consumer adoption must combine the leaf executor hash with host artifact/executor identity and the opaque dependency policy with host configuration identity before persistence. The current `eino-agent` behavior that overwrites per-tool executor identity with only the component artifact hash is not sufficient.

## Factory behavior

### Workspace definitions

- Require `Instance.WorkspaceRoot` to be non-empty and absolute.
- Resolve the root with `filepath.EvalSymlinks`, require the result to equal the cleaned supplied path, and require `os.Stat` to report a directory.
- Reject nonexistent paths, regular files, symlink aliases, and roots removed after catalog creation but before materialization.
- Document that these are structural usability checks. The host still owns authorization and admission for the canonical root.
- Pass that exact string to the leaf constructor. Do not read process cwd, consult a registry, or look up later options.
- Check `ctx.Err()` before construction and return it unchanged.
- Verify the captured shell or search executable fingerprint before calling those constructors.

### Static definitions

- Ignore an empty `Instance.WorkspaceRoot`; static tools do not acquire workspace authority from it.
- Check `ctx.Err()` before construction.
- Construct from the captured URL-fetch, user-interaction, or tracker dependency only.

### Atomicity

- `Standard` has no external side effects. It either returns the complete valid slice or `nil, error`.
- A definition factory does not register itself or mutate catalog state. It either returns one fully constructed leaf tool or `nil, error`.
- A host that mounts several translated definitions must provide its own transaction. The package example must make this ownership explicit.

## Retry and concurrency invariants

- `RetrySafe=true` denotes no intended external mutation: file read/list, glob, search, and URL fetch.
- All mutation, shell, user-interaction, and tracker operations remain non-retry-safe.
- Every workspace definition has `Concurrent=false`. The host uses one locker keyed by canonical root across all such definitions.
- URL fetch has `Concurrent=true` under the documented `http.Client` concurrency contract.
- User interaction and tracker write have `Concurrent=false` because their injected dependencies and correlation semantics do not promise concurrent use.

## Tests and observable acceptance criteria

1. Enumerate with `Options{}` twice and compare every public scalar field and metadata value in order.
2. Enumerate with a tracker writer and assert the fixed 11-entry order. Enumerate without it and assert the fixed 10-entry order.
3. Assert all IDs and names are unique, every factory and metadata accessor is non-nil, every hash is lowercase SHA-256 hex, and every binding/safety tuple matches the overview table.
4. Maintain a golden table of schema and executor hashes. An intentional metadata or executor revision change must update only the expected entries.
5. Call every metadata accessor without an instance or workspace and validate its Eino schema.
6. Materialize every definition, including URL fetch and user interaction, twice and assert distinct concrete tool pointers. Allow only the documented injected dependencies to be shared.
7. Use table-driven behavioral tests for every workspace definition with two root-distinguishing fixtures. Exercise read, write, edit, list, glob, search, apply-patch, and shell through safe root-observable calls; assert each invocation sees or changes only its supplied root. Use `pwd` or a root-local marker for shell.
8. Keep a dedicated file-write mutation test that writes the same relative path through tools bound to two roots and proves content isolation.
9. Mutate the caller's shell environment slice and option pointers after `Standard`; assert factories retain the captured values.
10. Save and restore `os.Stdin` and `os.Stderr` in a non-parallel CLI test. Build the catalog, reassign those globals, materialize user interaction, and prove it uses the originally captured streams.
11. Inject a distinguishable HTTP client, mutate or reassign the caller's `URLFetchOptions` container after `Standard`, and prove materialized URL-fetch tools use the captured client.
12. Verify empty user surface defaults to MCP and an invalid surface returns `nil, error`.
13. Verify a negative shell output cap returns `nil, error`.
14. Verify empty, relative, nonexistent, regular-file, symlink-alias, and removed-before-materialization workspace roots return errors for every workspace definition.
15. Verify a canceled context prevents both workspace and static construction.
16. Verify a nil tracker writer omits only tracker write and a configured writer produces working fresh tracker tools.
17. Supply typed-nil pointer implementations for `tracker.CloseWriter`, CLI `io.Reader`, and CLI `io.Writer`; assert `Standard` returns `nil, error` and no invocation can panic.
18. Change PATH and process environment after `Standard`; prove search and shell use their captured absolute executables and environments.
19. Replace a test executable after `Standard`; prove materialization fails with identity drift. Restore global environment in cleanup and do not run these tests in parallel.
20. Cross-compile the public catalog package and tests for Windows. In a Windows CI job, run `go test ./catalog` and assert the portable types compile while `Standard` returns the documented unsupported-platform error and no definitions.

## Dependencies, risks, and exclusions

- Depends on all metadata builders in work package 1.
- The public contract is intentionally consumer-neutral. Do not expose `composition.ToolRegistration`, host global/session scope types, keyed lockers, permission policy, retention, artifact/config hashes, or run-plan lifecycle.
- `BindingStatic` means no workspace root is supplied to the leaf constructor. It does not mean permission-free, filesystem-free, or globally mounted; URL fetch can reach absolute file paths and HTTPS endpoints.
- Direct `search.New` calls with no options preserve the current lookup behavior for leaf-package users. Catalog-created search tools always receive pinned options and `--no-config` prevents later `RIPGREP_CONFIG_PATH` changes from altering execution.
- Structural workspace validation can race with deletion after factory return. The host must retain workspace lifetime for the frozen run plan; the catalog guarantees only that the root is usable and canonical when the factory returns.
- Never include raw environment entries, executable contents, or injected dependency details in errors, golden fixtures, documentation, or logs. Publish only the specified digests and redacted identifiers.
- Do not claim deep immutability for injected interfaces. The catalog snapshots option containers and returns fresh leaf objects; the host owns the lifecycle and concurrency contract of supplied dependencies.
- Do not add a bulk materializer or mutable catalog registry. The returned slice is configuration for translation by the host.
