# Big Change Planning with Beads

## Agent Instructions

You are an expert software architect creating a comprehensive task breakdown for a change to an existing codebase. This task graph will be executed by AI agents working in parallel, coordinated through MCP Agent Mail with file reservations to prevent conflicts.

<quality_expectations>
Create a thorough, production-ready task graph. Include all necessary analysis, preparation, implementation, testing, and documentation tasks. Go beyond the basics -- consider edge cases, error handling, security considerations, backwards compatibility, and integration points. Each task should be specific enough for an agent to execute independently without ambiguity.
</quality_expectations>

<critical_constraint>
You must NOT implement any of the changes yourself. Your ONLY output is a bash shell script containing `bd create` and `bd dep add` commands. Do NOT use `bd add` -- the correct command is `bd create`. Do not write code. Do not create files other than the shell script. Do not modify existing files. Read and analyze the codebase, then produce the script.
</critical_constraint>

## Change Information

### Change Type
NEW_FEATURE

### Description
Add reusable coding-agent leaf-tool parity for `eino-agent` in `eino-tools`: a dedicated `glob` path-discovery tool, backward-compatible line-windowed `file_read`, a multi-file `apply_patch` tool, richer `search` controls, and explicit concurrency/ownership documentation.

### Links to Relevant Documentation
- Request: `.agents/requests/2026-06-26-coding-agent-tool-parity-for-eino-agent.md`
- Consumer plan: `~/git/eino-agent/docs/prompts/eino-agent-go-runtime-for-ag-ui-and-datadog.md`
- Existing docs: `README.md`, `docs/inventory/*.md`, `docs/adr/*.md`

### Affected Areas
- `fileops/`: extend `file_read` schema/result/runtime and tests while preserving plain `{path}` behavior.
- `search/`: extend ripgrep-backed search args/results/tests without changing the `search` tool name.
- `glob/`: add new package and `glob` model-facing tool.
- `applypatch/`: add new package and `apply_patch` model-facing tool.
- `docs/inventory/`, `docs/adr/`, `README.md`, `CHANGELOG.md`: document new tools, changed contracts, patch grammar, serialization, and ownership decisions.
- `go.mod`, `go.sum`: add doublestar dependency for glob matching.

### Success Criteria
- `eino-tools` exposes `glob`, upgraded `file_read`, `apply_patch`, and richer `search` controls with stable result envelopes and RawJSON preservation.
- Existing `file_read`, `file_list`, `file_edit`, and `search` calls remain compatible.
- Tests cover happy paths, duplicate top-level key rejection, schema shape/fresh copies, path/symlink escapes, truncation, binary handling where applicable, patch preflight failures, and all-or-partial patch semantics.
- Documentation states that `eino-agent` must serialize workspace filesystem tools per workspace root.
- Documentation states that `web_search` and LSP/diagnostics schemas belong to `eino-agent`, not this leaf-tool module.
- `go test ./...` and `go vet ./...` pass; `golangci-lint run` passes if configured and available.

### Constraints
- Preserve current Eino tool conventions: `Info(ctx)`, `InvokableRun(ctx, argsJSON, ...tool.Option)`, fresh schema helpers, duplicate top-level key rejection, stable result envelopes, and RawJSON preservation.
- Keep telemetry, permission prompting, subagents, task state, skills, output retention, AG-UI media attachments, and Datadog observability out of `eino-tools`.
- Do not add formatter, LSP, watcher, git, or telemetry side effects to `apply_patch`.
- Use package-level documentation rather than hidden runtime locks for the current concurrency contract.
- Skip optional GPT review for this saved prompt in non-interactive implementation sessions.

---

## Your Task

Analyze this codebase change and create a comprehensive **Beads task graph** using the `bd` CLI. Beads provides dependency-aware, conflict-free task management for multi-agent execution.

Before creating the task graph, you MUST first analyze the affected areas of the codebase:

1. Check `docs/specs/` and `docs/adr/` for existing architectural decisions
2. Examine the directory/module structure of the affected areas listed above
3. Identify key interfaces, APIs, and integration points that must be preserved
4. Note existing test patterns and coverage in the affected areas
5. Assess risk areas where changes could break existing functionality

Use your analysis to make each bead specific -- reference actual file paths, module names, and patterns you observed.

Then generate a shell script that creates the complete task graph.

**IMPORTANT: Your ONLY deliverable is a bash shell script with `bd create` commands. Not an implementation plan. Not a design document. Not a code review. A runnable `.sh` script.**

---

## Output Format

Generate a shell script that creates the full task graph. The script should:

1. **Initialize Beads** (if not already initialized)
2. **Create all beads** with appropriate priorities
3. **Establish dependencies** between beads
4. **Add labels** for phase grouping

### Example Output

```bash
#!/bin/bash
# Project: eino-tools
# Change: Coding-agent tool parity for eino-agent
# Generated: 2026-06-26

set -e

if [ ! -d ".beads" ]; then
    bd init
fi

echo "Creating change beads..."
```

---

## Verification Steps

After generating the script:

1. **Run it**: `chmod +x setup-beads.sh && ./setup-beads.sh`
2. **Check ready work**: `bd ready` should show initial analysis/prep tasks
