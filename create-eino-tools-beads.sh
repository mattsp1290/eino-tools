#!/bin/bash
# Project: eino-tools
# Generated: 2026-05-25

set -euo pipefail

if [ ! -d ".beads" ]; then
  bd init
fi

echo "Creating eino-tools v1 task graph..."

# ========================================
# Phase 0: Preflight Gates
# ========================================

PREFLIGHT_GO=$(bd create "Reconcile Go baseline across shared-module family" \
  -d "Compare this repo, ~/git/beads-go planning docs, ~/git/local-symphony/go.mod, ~/git/local-symphony/.golangci.yml, and the planned eino-providers baseline. Decide the Go directive and CI Go version before any extraction work. Do not assume Go 1.26 until this closes." \
  -p 0 -l preflight --silent)

PREFLIGHT_EINO=$(bd create "Confirm and pin cloudwego/eino minor version" \
  -d "Verify the cloudwego/eino minor version expected by eino-providers, target v0.8.x if compatible, and document the selected minor for all eino-tools packages and CI." \
  -p 0 -l preflight --silent)

PREFLIGHT_BEADSGO=$(bd create "Verify beads-go tag and Close API surface" \
  -d "Confirm github.com/mattsp1290/beads-go is tagged or otherwise importable and exposes the SDK surface needed by tracker/beads Close integration. Record the exact version or commit." \
  -p 0 -l preflight --silent)

PREFLIGHT_BD_NPM=$(bd create "Verify upstream bd npm package name for CI" \
  -d "Check upstream gastownhall/beads package.json and confirm whether the global install command is npm install -g @beads/bd. Record the verified package and bd --version behavior for integration CI." \
  -p 0 -l preflight --silent)

PREFLIGHT_SYMPHONY=$(bd create "Decide local-symphony adoption sequencing" \
  -d "Decide whether the local-symphony deletion PR includes Go/eino dependency bumps or depends on a separate completed PR. Record required branch or tag constraints for the final acceptance gate." \
  -p 0 -l preflight --silent)

# ========================================
# Phase 1: Public API and ADRs
# ========================================

ADR_RESULT=$(bd create "Write ADR 0001 for result Outcome enum contract" \
  -d "Create docs/adr/0001-result-outcome-enum.md documenting why dispatcher.ToolOutcome is replaced by result.Outcome, enum stability expectations, validation behavior, and consumer shim strategy." \
  -p 1 -l api --silent)

ADR_SHELL=$(bd create "Write ADR 0002 for shell options and sandbox boundary" \
  -d "Create docs/adr/0002-shell-options-and-sandbox.md documenting preserved cmd/timeout_seconds schema, intentional sh -lc execution, constructor-level Env/ShellBinary/OutputCapBytes, and caller-owned sandboxing." \
  -p 1 -l api --silent)

ADR_TRACKER_CLOSE=$(bd create "Write ADR 0003 for tracker CloseWriter v0.1 scope" \
  -d "Create docs/adr/0003-tracker-close-writer-v0.1.md documenting close-only interface, deferred Comment/Transition/LinkPR/IssueState, and pre-v1.0 breaking-change policy." \
  -p 1 -l api --silent)

ADR_TRACKER_BEADS=$(bd create "Write ADR 0004 for beads SDK adapter instead of os/exec" \
  -d "Create docs/adr/0004-tracker-beads-via-sdk-not-exec.md documenting why tracker/beads imports beads-go SDK and must not shell out to bd directly." \
  -p 1 -l api --silent)

ADR_SEARCH=$(bd create "Write ADR 0005 for search backend choice" \
  -d "Create docs/adr/0005-search-backend-ripgrep-vs-pure-go.md. Default to preserving current rg --json -e behavior unless a future design explicitly chooses a pure-Go rewrite with schema and prompt migration." \
  -p 1 -l api --silent)

bd dep add "$ADR_RESULT" "$PREFLIGHT_GO"
bd dep add "$ADR_SHELL" "$PREFLIGHT_GO"
bd dep add "$ADR_TRACKER_CLOSE" "$PREFLIGHT_BEADSGO"
bd dep add "$ADR_TRACKER_BEADS" "$PREFLIGHT_BEADSGO"
bd dep add "$ADR_SEARCH" "$PREFLIGHT_EINO"

# ========================================
# Phase 2: Repository Setup
# ========================================

SETUP_GOMOD=$(bd create "Initialize Go module and baseline dependencies" \
  -d "Create go.mod for github.com/mattsp1290/eino-tools using the reconciled Go baseline. Add github.com/cloudwego/eino and github.com/google/jsonschema-go. Do not add beads-go until tracker/beads lands." \
  -p 0 -l setup --silent)

SETUP_LICENSE=$(bd create "Install MIT LICENSE" \
  -d "Replace any placeholder LICENSE with standard MIT license text for Matt Spurlin, year 2026." \
  -p 1 -l setup --silent)

SETUP_LINT=$(bd create "Add golangci-lint configuration" \
  -d "Copy ~/git/local-symphony/.golangci.yml as the baseline, retarget the Go version to the reconciled baseline, and remove local-symphony-specific path exclusions." \
  -p 1 -l setup --silent)

SETUP_CI_UNIT=$(bd create "Add unit CI workflow" \
  -d "Create .github/workflows/ci.yml unit job with go mod tidy --diff, go vet ./..., golangci-lint run, go test -race ./..., and dependency hygiene checks." \
  -p 1 -l setup --silent)

SETUP_CHANGELOG=$(bd create "Create hand-curated CHANGELOG" \
  -d "Create CHANGELOG.md starting with Unreleased. Include pre-v1.0 semver caveat and migration-note requirement for breaking changes." \
  -p 2 -l setup --silent)

SETUP_README=$(bd create "Create no-marketing README skeleton" \
  -d "Create README.md with one-sentence purpose, one concise example per tool sub-package, consumer project links, security notes, and no marketing copy." \
  -p 2 -l setup --silent)

bd dep add "$SETUP_GOMOD" "$PREFLIGHT_GO"
bd dep add "$SETUP_GOMOD" "$PREFLIGHT_EINO"
bd dep add "$SETUP_LINT" "$SETUP_GOMOD"
bd dep add "$SETUP_CI_UNIT" "$SETUP_GOMOD"
bd dep add "$SETUP_CI_UNIT" "$SETUP_LINT"
bd dep add "$SETUP_README" "$SETUP_GOMOD"

# ========================================
# Phase 3: Core Result Package
# ========================================

CORE_RESULT_API=$(bd create "Implement result Outcome package" \
  -d "Create result/doc.go and result/outcome.go with Outcome type, constants succeeded/failed/timed_out/rejected, validation helpers, and enum stability documentation. No dispatcher imports." \
  -p 0 -l core --silent)

CORE_RESULT_TESTS=$(bd create "Test result Outcome validation and stability" \
  -d "Add tests for valid and invalid Outcome values, string behavior, zero-value handling, and documented enum constants." \
  -p 1 -l core --silent)

CORE_JSON_HELPERS=$(bd create "Add shared JSON decoding helpers if locally justified" \
  -d "If extraction shows duplicated schema-safe decoding, add small unexported or internal helper code for duplicate top-level key rejection and RawJSON preservation. Keep package boundaries clean and avoid broad abstractions unless reused by multiple tools." \
  -p 2 -l core --silent)

bd dep add "$CORE_RESULT_API" "$SETUP_GOMOD"
bd dep add "$CORE_RESULT_API" "$ADR_RESULT"
bd dep add "$CORE_RESULT_TESTS" "$CORE_RESULT_API"
bd dep add "$CORE_JSON_HELPERS" "$CORE_RESULT_API"

# ========================================
# Phase 4: fileops Extraction
# ========================================

FILEOPS_INVENTORY=$(bd create "Inventory local-symphony fileops source and tests" \
  -d "Read ~/git/local-symphony/internal/worker/tools/fileops and identify public constants, constructors, schemas, result envelopes, path helpers, test fixtures, and dispatcher coupling to replace." \
  -p 0 -l feature-fileops --silent)

FILEOPS_PORT_CORE=$(bd create "Extract fileops workspace boundary and shared helpers" \
  -d "Create fileops/doc.go and fileops/fileops.go. Preserve canonicalizeWorkspace, resolveExisting, resolveWritable, symlink handling, workspace-rooted security semantics, and current error categories." \
  -p 0 -l feature-fileops --silent)

FILEOPS_PORT_TOOLS=$(bd create "Extract fileops Read Write Edit List tools" \
  -d "Create fileops/read.go, write.go, edit.go, and list.go as eino tools. Preserve Name constants, Schema fresh-copy behavior, InvokableRun signature, additionalProperties:false, parse errors, and duplicate key rejection." \
  -p 0 -l feature-fileops --silent)

FILEOPS_RESULT_CONTRACT=$(bd create "Document and test fileops result forward compatibility" \
  -d "Add package-level result struct docs. Preserve unknown top-level JSON fields in RawJSON. Test unknown field decode succeeds and survives re-encoding or inspection." \
  -p 1 -l feature-fileops --silent)

FILEOPS_SECURITY_TESTS=$(bd create "Port and expand fileops path security tests" \
  -d "Move path traversal, absolute path, symlink escape, missing path, not_found, validation, and path_escape tests. Assert no operation touches files outside the workspace." \
  -p 0 -l feature-fileops --silent)

FILEOPS_SCHEMA_TESTS=$(bd create "Add fileops schema ABI tests" \
  -d "Test each Schema call returns a fresh copy, duplicate top-level JSON keys are rejected before execution, and current model-facing JSON shape stays source-compatible with local-symphony." \
  -p 1 -l feature-fileops --silent)

bd dep add "$FILEOPS_INVENTORY" "$CORE_RESULT_API"
bd dep add "$FILEOPS_PORT_CORE" "$FILEOPS_INVENTORY"
bd dep add "$FILEOPS_PORT_TOOLS" "$FILEOPS_PORT_CORE"
bd dep add "$FILEOPS_RESULT_CONTRACT" "$FILEOPS_PORT_TOOLS"
bd dep add "$FILEOPS_SECURITY_TESTS" "$FILEOPS_PORT_TOOLS"
bd dep add "$FILEOPS_SCHEMA_TESTS" "$FILEOPS_PORT_TOOLS"

# ========================================
# Phase 5: search Extraction
# ========================================

SEARCH_INVENTORY=$(bd create "Inventory local-symphony search source and tests" \
  -d "Read ~/git/local-symphony/internal/worker/tools/search and identify rg invocation, schema, output caps, path resolver semantics, result envelope, and dispatcher coupling." \
  -p 0 -l feature-search --silent)

SEARCH_PORT=$(bd create "Extract ripgrep-backed search tool" \
  -d "Create search/doc.go and search/search.go. Preserve rg --json -e pattern behavior, workspace EvalSymlinks constructor/resolver semantics, output cap behavior, Name constant, Schema fresh copy, and InvokableRun ABI." \
  -p 0 -l feature-search --silent)

SEARCH_SECURITY_TESTS=$(bd create "Port search workspace and path security tests" \
  -d "Add tests for traversal, symlink escape, absolute path validation, missing path behavior, path_escape category, and no filesystem access outside workspace." \
  -p 0 -l feature-search --silent)

SEARCH_SCHEMA_TESTS=$(bd create "Add search schema and duplicate-key ABI tests" \
  -d "Test additionalProperties:false, duplicate top-level JSON key rejection before execution, schema fresh-copy behavior, parse errors, and local-symphony source compatibility." \
  -p 1 -l feature-search --silent)

SEARCH_RESULT_CONTRACT=$(bd create "Document and test search result forward compatibility" \
  -d "Document per-tool result struct compatibility and add RawJSON unknown-field preservation tests." \
  -p 1 -l feature-search --silent)

bd dep add "$SEARCH_INVENTORY" "$CORE_RESULT_API"
bd dep add "$SEARCH_INVENTORY" "$ADR_SEARCH"
bd dep add "$SEARCH_PORT" "$SEARCH_INVENTORY"
bd dep add "$SEARCH_SECURITY_TESTS" "$SEARCH_PORT"
bd dep add "$SEARCH_SCHEMA_TESTS" "$SEARCH_PORT"
bd dep add "$SEARCH_RESULT_CONTRACT" "$SEARCH_PORT"

# ========================================
# Phase 6: shell Extraction
# ========================================

SHELL_INVENTORY=$(bd create "Inventory local-symphony shell source and tests" \
  -d "Read ~/git/local-symphony/internal/worker/tools/shell and identify sh -lc behavior, timeout semantics, env assumptions, output caps, schema, result envelope, and dispatcher coupling." \
  -p 0 -l feature-shell --silent)

SHELL_OPTIONS=$(bd create "Design shell constructor options" \
  -d "Create shell/options.go with Options{Env, ShellBinary, OutputCapBytes} and conservative defaults preserving current behavior. Keep cwd as configured workspace root." \
  -p 0 -l feature-shell --silent)

SHELL_PORT=$(bd create "Extract shell tool with intentional exec boundary" \
  -d "Create shell/doc.go and shell/shell.go. Preserve cmd/timeout_seconds schema, sh -lc command execution, Name constant, Schema fresh copy, InvokableRun ABI, timeout behavior, and result categories. Keep #nosec G204 with an accurate comment about caller-contained sandboxing." \
  -p 0 -l feature-shell --silent)

SHELL_TESTS=$(bd create "Port and expand shell execution tests" \
  -d "Add tests for successful commands, nonzero exits, timeouts, output cap truncation, env options, shell binary option, cwd workspace behavior, parse errors, and duplicate top-level key rejection." \
  -p 0 -l feature-shell --silent)

SHELL_RESULT_CONTRACT=$(bd create "Document and test shell result forward compatibility" \
  -d "Document shell result struct compatibility and test unknown top-level JSON fields are preserved in RawJSON." \
  -p 1 -l feature-shell --silent)

bd dep add "$SHELL_INVENTORY" "$CORE_RESULT_API"
bd dep add "$SHELL_INVENTORY" "$ADR_SHELL"
bd dep add "$SHELL_OPTIONS" "$SHELL_INVENTORY"
bd dep add "$SHELL_PORT" "$SHELL_OPTIONS"
bd dep add "$SHELL_TESTS" "$SHELL_PORT"
bd dep add "$SHELL_RESULT_CONTRACT" "$SHELL_PORT"

# ========================================
# Phase 7: tracker and tracker/beads
# ========================================

TRACKER_INVENTORY=$(bd create "Inventory local-symphony trackerwrite execution path" \
  -d "Read ~/git/local-symphony/internal/worker/tools/trackerwrite and ~/git/local-symphony/internal/tracker/tracker.go. Identify only the close execution path, unsupported_op behavior, schema, result envelope, and dispatcher coupling." \
  -p 0 -l feature-tracker --silent)

TRACKER_INTERFACE=$(bd create "Implement tracker CloseWriter interface" \
  -d "Create tracker/doc.go and tracker/writer.go with CloseWriter interface: Close(ctx context.Context, id, reason string) error. Do not lift TrackerReader, TrackerWriter, Tracker, core.IssueState, or other local-symphony abstractions." \
  -p 0 -l feature-tracker --silent)

TRACKERWRITE_TOOL=$(bd create "Extract tracker_write tool around CloseWriter" \
  -d "Create tracker_write-compatible tool package surface as planned for this repo. Preserve comment, transition, close, and link_pr model-facing args, execute only close, and preserve unsupported_op envelopes for non-close ops." \
  -p 0 -l feature-tracker --silent)

TRACKER_BEADS_ADAPTER=$(bd create "Implement tracker/beads SDK adapter" \
  -d "Create tracker/beads/doc.go and adapter.go importing github.com/mattsp1290/beads-go/beads. Adapt *beads.Client or a narrow local Close interface to tracker.CloseWriter. The adapter file must have zero os/exec import." \
  -p 0 -l feature-tracker --silent)

TRACKER_UNIT_TESTS=$(bd create "Add tracker and tracker/beads unit tests with replay fake" \
  -d "Use a fake implementing a small local interface such as Close(context.Context,string,string) error. Test close success, close error propagation, unsupported_op behavior, schema ABI, duplicate key rejection, and RawJSON unknown-field preservation. Do not require bd binary in unit tests." \
  -p 0 -l feature-tracker --silent)

TRACKER_INTEGRATION_TEST=$(bd create "Add tracker/beads integration test under build tag" \
  -d "Create go test -tags=integration coverage that uses temp dir, bd init, real beads-go SDK, and Close end to end through the real bd binary. Keep skipped unless integration tag and bd are available." \
  -p 1 -l feature-tracker --silent)

CI_BEADS_INTEGRATION=$(bd create "Add tracker/beads integration CI job" \
  -d "Extend GitHub Actions with a separate job that sets up Node, installs the verified bd npm package globally, verifies bd --version, then runs go test -tags=integration ./tracker/beads/...." \
  -p 1 -l feature-tracker --silent)

bd dep add "$TRACKER_INVENTORY" "$CORE_RESULT_API"
bd dep add "$TRACKER_INVENTORY" "$ADR_TRACKER_CLOSE"
bd dep add "$TRACKER_INTERFACE" "$TRACKER_INVENTORY"
bd dep add "$TRACKERWRITE_TOOL" "$TRACKER_INTERFACE"
bd dep add "$TRACKER_BEADS_ADAPTER" "$TRACKER_INTERFACE"
bd dep add "$TRACKER_BEADS_ADAPTER" "$ADR_TRACKER_BEADS"
bd dep add "$TRACKER_BEADS_ADAPTER" "$PREFLIGHT_BEADSGO"
bd dep add "$TRACKER_UNIT_TESTS" "$TRACKERWRITE_TOOL"
bd dep add "$TRACKER_UNIT_TESTS" "$TRACKER_BEADS_ADAPTER"
bd dep add "$TRACKER_INTEGRATION_TEST" "$TRACKER_BEADS_ADAPTER"
bd dep add "$TRACKER_INTEGRATION_TEST" "$PREFLIGHT_BD_NPM"
bd dep add "$CI_BEADS_INTEGRATION" "$TRACKER_INTEGRATION_TEST"
bd dep add "$CI_BEADS_INTEGRATION" "$SETUP_CI_UNIT"

# ========================================
# Phase 8: Dependency Hygiene and Cross-Package Checks
# ========================================

HYGIENE_FORBIDDEN_IMPORTS=$(bd create "Add forbidden import hygiene checks" \
  -d "Add scripted or test-based go list -deps checks for every public package. Assert fileops/result/tracker do not import os/exec, no package imports cloudwego/eino-ext, no package imports telemetry, log, slog, or auth deps, and only tracker/beads pulls beads-go." \
  -p 0 -l hygiene --silent)

HYGIENE_PACKAGE_ISOLATION=$(bd create "Verify per-package dependency isolation" \
  -d "Assert consumers importing only fileops do not transitively pull beads-go, os/exec, telemetry, logging, auth, or eino-ext. Repeat explicit forbidden dependency sets for result, search, shell, tracker, and tracker/beads." \
  -p 0 -l hygiene --silent)

HYGIENE_DISPATCHER=$(bd create "Verify no internal dispatcher imports remain" \
  -d "Add grep or go list checks proving no extracted package imports ~/git/local-symphony/internal/dispatcher or references dispatcher.ToolOutcome." \
  -p 0 -l hygiene --silent)

HYGIENE_TIDY=$(bd create "Run tidy, vet, lint, race tests, and dependency checks locally" \
  -d "Run go mod tidy, go mod tidy --diff if available, go vet ./..., golangci-lint run, go test -race ./..., and all dependency hygiene checks. Fix issues in the owning package tasks." \
  -p 0 -l hygiene --silent)

bd dep add "$HYGIENE_FORBIDDEN_IMPORTS" "$FILEOPS_PORT_TOOLS"
bd dep add "$HYGIENE_FORBIDDEN_IMPORTS" "$SEARCH_PORT"
bd dep add "$HYGIENE_FORBIDDEN_IMPORTS" "$SHELL_PORT"
bd dep add "$HYGIENE_FORBIDDEN_IMPORTS" "$TRACKER_BEADS_ADAPTER"
bd dep add "$HYGIENE_PACKAGE_ISOLATION" "$HYGIENE_FORBIDDEN_IMPORTS"
bd dep add "$HYGIENE_DISPATCHER" "$CORE_RESULT_API"
bd dep add "$HYGIENE_DISPATCHER" "$FILEOPS_PORT_TOOLS"
bd dep add "$HYGIENE_DISPATCHER" "$SEARCH_PORT"
bd dep add "$HYGIENE_DISPATCHER" "$SHELL_PORT"
bd dep add "$HYGIENE_DISPATCHER" "$TRACKERWRITE_TOOL"
bd dep add "$HYGIENE_TIDY" "$HYGIENE_PACKAGE_ISOLATION"
bd dep add "$HYGIENE_TIDY" "$HYGIENE_DISPATCHER"
bd dep add "$HYGIENE_TIDY" "$CORE_RESULT_TESTS"
bd dep add "$HYGIENE_TIDY" "$FILEOPS_SECURITY_TESTS"
bd dep add "$HYGIENE_TIDY" "$SEARCH_SECURITY_TESTS"
bd dep add "$HYGIENE_TIDY" "$SHELL_TESTS"
bd dep add "$HYGIENE_TIDY" "$TRACKER_UNIT_TESTS"

# ========================================
# Phase 9: Documentation and Release Prep
# ========================================

DOCS_COMPLETE_README=$(bd create "Finalize README examples and consumer guidance" \
  -d "Update README with accurate examples for result, fileops, search, shell, tracker, and tracker/beads. Include honest shell sandbox note and local-symphony consumer link." \
  -p 2 -l docs --silent)

DOCS_CHANGELOG_UNRELEASED=$(bd create "Update CHANGELOG for v0.1.0 extraction" \
  -d "Document initial v0.1.0 surface, pre-v1.0 breaking-change policy, deferred tracker operations, and any migration notes discovered during extraction." \
  -p 2 -l docs --silent)

RELEASE_DRYRUN=$(bd create "Prepare v0.1.0 release dry run" \
  -d "Confirm module path, package docs, tags plan, CI status, and changelog state. Do not tag until the local-symphony acceptance PR passes." \
  -p 2 -l release --silent)

bd dep add "$DOCS_COMPLETE_README" "$FILEOPS_PORT_TOOLS"
bd dep add "$DOCS_COMPLETE_README" "$SEARCH_PORT"
bd dep add "$DOCS_COMPLETE_README" "$SHELL_PORT"
bd dep add "$DOCS_COMPLETE_README" "$TRACKER_BEADS_ADAPTER"
bd dep add "$DOCS_CHANGELOG_UNRELEASED" "$DOCS_COMPLETE_README"
bd dep add "$RELEASE_DRYRUN" "$HYGIENE_TIDY"
bd dep add "$RELEASE_DRYRUN" "$CI_BEADS_INTEGRATION"
bd dep add "$RELEASE_DRYRUN" "$DOCS_CHANGELOG_UNRELEASED"

# ========================================
# Phase 10: local-symphony Acceptance PR
# ========================================

SYMPHONY_WRAPPERS=$(bd create "Create local-symphony thin wrappers for extracted tools" \
  -d "In ~/git/local-symphony, replace internal/worker/tools/{fileops,search,shell,trackerwrite}/ implementations with thin wrappers importing eino-tools sub-packages. Add a small dispatcher.ToolOutcome to result.Outcome shim or type alias per adoption decision." \
  -p 0 -l acceptance --silent)

SYMPHONY_DELETE_OLD=$(bd create "Delete old local-symphony tool implementations and tests" \
  -d "Delete internal/worker/tools/{fileops,search,shell,trackerwrite}/ source and corresponding migrated test files once wrappers are in place. Preserve only intentional wrapper/adoption code." \
  -p 0 -l acceptance --silent)

SYMPHONY_TESTS=$(bd create "Run local-symphony full test suite against wrappers" \
  -d "Run all existing local-symphony tests required by the repo, including worker tool tests, dispatcher integration tests, lint/vet as applicable, and any Go/eino bump validation from the preflight sequencing decision." \
  -p 0 -l acceptance --silent)

SYMPHONY_PR=$(bd create "Open local-symphony deletion PR acceptance gate" \
  -d "Open the PR that proves v1 is done: old tool packages deleted, thin wrappers import eino-tools, outcome shim present, all local-symphony tests pass, and deployment sandbox files remain unchanged." \
  -p 0 -l acceptance --silent)

bd dep add "$SYMPHONY_WRAPPERS" "$RELEASE_DRYRUN"
bd dep add "$SYMPHONY_WRAPPERS" "$PREFLIGHT_SYMPHONY"
bd dep add "$SYMPHONY_DELETE_OLD" "$SYMPHONY_WRAPPERS"
bd dep add "$SYMPHONY_TESTS" "$SYMPHONY_DELETE_OLD"
bd dep add "$SYMPHONY_PR" "$SYMPHONY_TESTS"

echo ""
echo "Bead graph created. Useful commands:"
echo "  bd ready"
echo "  bd list"
echo "  bd graph"
