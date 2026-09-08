# Work package 2: Catalog capture and execution identity

Prerequisite: work package 1 public options and validation. Goal: the standard catalog validates and captures startup policy before mounting, and identity reflects the policy actually executed. No feature flag or data migration is needed.

## Capture changes

In existing `catalog/executable_unix.go`, call the proposed `shell.Options.Validate` from `captureShellOptions` before executable capture. Normalize empty startup mode to the proposed `shell.StartupModeLogin`, then retain it in the options struct copied into the factory. Do not call `shell.New` with a fake root to validate options. Preserve atomic errors: `Standard` returns nil definitions plus an error for unsupported modes, even if the shell tool is never materialized.

Replace the shell Env cloning expression with a copy that preserves nil versus nonnil-empty, such as `slices.Clone`. `captureExecutable` must inherit only when the original requested environment was nil. Its existing `normalizeEnvironment` already returns a nonnil empty output when given an empty input. Limit this fix to shell capture; a similar search issue can be tracked separately without expanding the startup-policy change.

Leave executable resolution, invocation aliases, target hashing, UTF-8 checks and materialization drift checks intact. `catalog.Options.ShellOptions` already carries the additive mode field; no parallel catalog setting or new mount API is necessary. Preserve capture timing: catalog nil Env is snapshotted at `Standard`, whereas leaf nil Env inherits per execution as before.

In `catalog/catalog_unix.go`, keep the existing `shellFactory` boundary: captured policy → verified executable → `shell.New(root, shellOptions)`. No mutable exported setter, retained caller Options pointer, or per-invocation reread of host options is allowed. The host still owns immutable executable artifacts after materialization; this work does not repair the existing verification/use race.

## Identity changes

In existing `catalog/identity.go`, add proposed internal `shellExecutionPolicy` with fixed JSON fields `startup_mode` and `output_cap_bytes`. The mode uses the normalized string; the cap uses the effective positive value. Add proposed optional `ShellPolicy *shellExecutionPolicy` with JSON tag `shell_policy,omitempty` to `executorIdentity`. Extend `executorHash` to accept the optional policy record, and all existing call sites/tests to pass it explicitly. In existing `catalog/catalog_unix.go`, add proposed internal `definitionSpec.shellPolicy` and carry it through `makeDefinition`.

Only the `IDShell` specification supplies this record. It must be built from the same captured `shellOptions` used by `shellFactory`, not the caller input or a second defaulting path. Bump `IDShell` executor revision from 1 to 2 because option interpretation and invocation semantics change. Preserve executor identity version v1 and exact JSON bytes for unrelated tools by omitting the new field when nil. This extension is explicit in ADR 0009; if implementation cannot preserve nil-record bytes, resolve that defect before merging instead of silently changing every identity.

The output cap is included because it affects observable execution policy and was previously absent. Binary invocation/target paths and environment remain in their existing identity fields. No raw environment values enter descriptions, schemas or response artifacts. Keep metadata independent of mode; update only the shell schema golden for the policy-neutral descriptions from package 1. Retain the baseline file-read revision 2 and its golden.

## Verification and acceptance

Extend `catalog/catalog_test.go`, `catalog/executable_test.go`, `catalog/identity_test.go`, and `catalog/leaf_metadata_test.go` where their existing responsibilities fit. New test symbols under these existing files are proposed. Audit all existing catalog fixtures that invoke a real shell and explicitly select non-login mode for ordinary host runs; default/login identity comparisons may construct without executing. Any real login variants use the package 1 test-only opt-in in the controlled CI container. Preserve production default assertions via capture/defaulting checks and the argv recorder.

- `Standard` with an invalid mode returns zero definitions and an error at capture. Provide valid search options to avoid an unrelated ripgrep lookup error masking this assertion. A recorder marker proves validation has launched no shell process.
- For identical synthetic executable bytes, paths and environment, default/empty/login modes produce equal executor hashes. Non-login differs. Non-login versus login schemas are identical within the new release. Reordered equivalent environment entries and repeated constructions remain deterministic. Different effective output caps differ; zero and the default cap are equal.
- Preserve `TestExecutorHashGolden` for nil policy. Add a synthetic policy golden to prove the exact JSON record extension, not only inequality. All unrelated schema/executor goldens stay unchanged.
- Create `Standard` with explicit non-login, mutable Env and custom cap. Mutate every input field, the Env backing slice and the caller's pointer after capture, then materialize twice. Decode shell results and assert the original environment, invocation mode and cap both times. Repeat mutation after first materialization to cover future factories.
- Exercise explicit empty Env with an absolute shell executable and a synthetic parent canary. After capture and invocation the canary stays absent. Nil Env captures the canary at `Standard`, and changing the parent afterward does not affect later factories. Avoid parallel tests when mutating process environment.
- Verify schema/Info fresh copies, instance independence, option mutation and executable drift tests still pass. Unknown model JSON policy fields cannot alter captured execution; assert this through real invocation as well as the metadata surface.

Run `go test -race ./shell ./catalog`. Keep the existing Windows `go test ./catalog` job; `Options.Validate` and mode declarations must compile without Unix process symbols. The mounted proof and persisted descriptor identity are mandatory in [package 3](03-mounted-verification-and-delivery.md); a leaf factory invocation alone does not satisfy mount acceptance.

## Documentation and rollback

Revise existing `docs/adr/0009-standard-tool-catalog-identity.md` to describe the optional shell policy record, normalization, shell revision 2, cap fingerprint and nil/empty environment semantics. State that changing shell metadata or executor hash changes persisted plan identities. Do not bypass resume drift checks: rebuild affected plans under the new artifact/config identity. Rollback means selecting an earlier artifact for new runs with freshly constructed plans; never mutate the captured mode of an existing plan. Restoring login mode may restore startup-file execution, so Ensemble policy rollback belongs to that host.
