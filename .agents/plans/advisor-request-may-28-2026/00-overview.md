# Plan: `userinteract` and `urlfetch` for eino-tools

**Request source:** `docs/requests/planner-tools-2026-05-28.md`  
**Status:** Staged for engineer review — no code written yet  
**Date:** 2026-05-28

## Purpose

The `planner` tool (in `github.com/mattsp1290/advisor`) needs two capabilities not yet present in eino-tools:

1. **`urlfetch`** — fetch the text content of a `file://` or `https://` URL and return it as a string
2. **`userinteract`** — ask the user a question and receive their typed answer; must work in both blocking (CLI) and non-blocking (MCP) contexts without blocking the MCP server

This plan documents the implementation scope, sequencing, key design decisions with recommendations, and required per-tool boilerplate. It is staged: write to `eino-tools` only after the engineer confirms the approach.

---

## Sequencing

**`urlfetch` first, then `userinteract`.**

`urlfetch` has no open design decisions. It is a good warm-up and likely to be ready for upstream immediately. `userinteract` has at least one design question (Outcome representation) that needs sign-off.

### Scoping fork — `userinteract` punt option

The request explicitly preserves this option: if `userinteract`'s surface-awareness adds more architectural weight than is acceptable for eino-tools at this time, the planner can implement it inline as `internal/planner/interact/interact.go` and propose contributing it upstream later. In that case, implement `urlfetch` only. Track the deferred contribution in a beads issue.

---

## Open questions requiring engineer sign-off

These must be answered before `userinteract` code is written. Full context is in `02-userinteract.md`.

**Q1 — Outcome representation:** Recommend a tool-local `Outcome` enum `{succeeded, pending, failed}` rather than importing `result.Outcome`. ADR 0001 closes the shared enum and each tool owns its result shape. The alternative (use `result.Outcome = "succeeded"` plus a `Pending bool` field) is more conventional but semantically awkward. **Which do you prefer?**

**Q2 — Stateless MCP design:** The plan models MCP interaction as stateless: the caller provides the answer via a follow-up tool call with `answer` non-empty in Args. The tool holds no cross-call state. Confirm this contract fits the planner's agent loop before implementing.

**Q3 — Injectable I/O:** The plan injects `io.Reader` (stdin) and `io.Writer` (stderr prompt) at construction, defaulting to `os.Stdin`/`os.Stderr`. This is what makes the "must not block in MCP mode" requirement testable. Confirm injection at construction (not per-call) is acceptable.

---

## Package locations

| Tool | Package import path |
|------|---------------------|
| `urlfetch` | `github.com/mattsp1290/eino-tools/urlfetch` |
| `userinteract` | `github.com/mattsp1290/eino-tools/userinteract` |

Both are new top-level packages. Neither requires changes to any existing eino-tools package.

---

## Quality gates (both tools)

- `go test -race ./...` passes
- `golangci-lint run` passes against repo `.golangci.yml`
- All tests use `t.Parallel()` and table-driven patterns
- `goimports` applied with local prefix `github.com/mattsp1290/eino-tools`

---

## Required boilerplate checklist (both tools)

Every new tool in this repo requires all of the following. Reproduced here so nothing is missed during implementation.

```
[ ] doc.go                     — package doc with usage guidelines
[ ] <tool>.go                  — main implementation
[ ] <tool>_test.go             — tests

Per-tool struct/function requirements:
[ ] const Name = "<name>"                    — model-facing tool name
[ ] type Args struct                          — JSON input, all json tags present
[ ] type Result struct { ... RawJSON json.RawMessage `json:"-"` }
[ ] func (r *Result) UnmarshalJSON(raw []byte) error   — captures RawJSON
[ ] type ResultError struct { Category, Message string }
[ ] const ErrCategory* = "..."               — at least one per error class
[ ] func (r Result) IsRetryable() bool
[ ] const schemaJSON = `{...}`               — embedded JSON Schema
[ ] func Schema() json.RawMessage            — returns bytes.Clone([]byte(schemaJSON))
[ ] type Tool struct                         — holds configuration
[ ] func New(...) (*Tool, error)             — constructor with validation
[ ] func (t *Tool) Run(ctx, args) Result     — core logic
[ ] func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error)
[ ] func (t *Tool) InvokableRun(ctx, argsJSON, ...tool.Option) (string, error)
[ ] var _ tool.InvokableTool = (*Tool)(nil)  — compile-time interface check

Docs:
[ ] docs/inventory/<tool>.md   — inventory entry
[ ] README.md entry            — package row in the tools table
```

---

## See also

- `01-urlfetch.md` — full implementation spec for `urlfetch`
- `02-userinteract.md` — full implementation spec for `userinteract`
- `docs/requests/planner-tools-2026-05-28.md` — original request (in-repo copy)
