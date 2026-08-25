# Work package 1: instance-independent leaf metadata

## Goal and prerequisites

Expose each existing Eino `ToolInfo` without constructing a leaf tool. Start from a green `make test` baseline. Do not change any model-visible metadata or execution behavior.

## Repository evidence

- Existing workspace-bound `Info` methods are in `fileops/read.go`, `fileops/write.go`, `fileops/edit.go`, `fileops/list.go`, `glob/glob.go`, `search/search.go`, `applypatch/applypatch.go`, and `shell/shell.go`.
- Existing static-tool `Info` methods are in `urlfetch/urlfetch.go`, `userinteract/userinteract.go`, and `trackerwrite/trackerwrite.go`.
- The methods do not use receiver state. They parse package-owned schema bytes and combine them with stable name and description literals.
- `fileops/fileops.go` already centralizes JSON-schema parsing in `buildToolInfo`.
- Every leaf package already tests its Eino ABI and schema-copy behavior.

## Exact change surface

Add these proposed exported functions at the existing `Info` insertion points:

| Existing file | Proposed symbol | Existing method that delegates |
|---|---|---|
| `fileops/read.go` | `ReadToolInfo() (*schema.ToolInfo, error)` | `(*ReadTool).Info` |
| `fileops/write.go` | `WriteToolInfo() (*schema.ToolInfo, error)` | `(*WriteTool).Info` |
| `fileops/edit.go` | `EditToolInfo() (*schema.ToolInfo, error)` | `(*EditTool).Info` |
| `fileops/list.go` | `ListToolInfo() (*schema.ToolInfo, error)` | `(*ListTool).Info` |
| `glob/glob.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |
| `search/search.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |
| `applypatch/applypatch.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |
| `shell/shell.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |
| `urlfetch/urlfetch.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |
| `userinteract/userinteract.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |
| `trackerwrite/trackerwrite.go` | `ToolInfo() (*schema.ToolInfo, error)` | `(*Tool).Info` |

Add focused tests to the existing package test files. A separate proposed test file is allowed when it keeps a large existing test file readable; its parent package directory already exists.

## Intended behavior and invariants

1. Each package-level builder returns a newly allocated `schema.ToolInfo` and newly parsed parameter schema on every call.
2. Each builder uses the existing exported name constant, existing description literal, and existing schema bytes.
3. Each existing receiver method ignores its context as it does today and delegates directly to the matching builder.
4. Metadata construction never validates, canonicalizes, opens, or otherwise observes a workspace path.
5. Metadata construction never applies shell, HTTP, terminal, tracker, or process-global defaults.
6. A caller can mutate one returned `ToolInfo` without affecting later calls.
7. WP1 leaves constructors, `Schema` functions, `InvokableRun` methods, and concrete tool types unchanged. WP2 later widens `search.New` with an optional execution-configuration argument while preserving no-option behavior.

## Error paths

- Preserve the existing package-prefixed schema-parse errors.
- Do not hide a malformed embedded schema. The package-level builder and receiver method must return the same error.
- Do not use nil receivers as a metadata API. The package-level function is the supported boundary.

## Tests and acceptance criteria

For every builder:

1. Call the builder twice and assert distinct `ToolInfo` pointers.
2. Assert the existing name and exact description remain unchanged.
3. Convert `ParamsOneOf` through `ToJSONSchema`, marshal it, and compare it with the package's exported schema JSON after semantic JSON normalization.
4. Mutate the first returned name, description, and JSON-schema object where practical; assert the second and a third call retain the original metadata.
5. Construct a real tool in a temporary workspace, call its receiver `Info`, and assert semantic equality with the package-level builder.

The package passes when all metadata is available without a root and every pre-existing leaf test remains green.

## Dependencies and risks

- Prerequisite: none beyond the current green baseline.
- This work package blocks catalog implementation because the catalog must not duplicate descriptions or construct fake tools.
- The main risk is accidental schema or description drift during extraction. Keep the original literals byte-for-byte and use semantic schema comparisons.

## Exclusions

- Do not consolidate all metadata into a new cross-package internal helper in this work package.
- Do not rename existing receiver methods or schema functions.
- Do not change descriptions to improve wording.
