# Extracting `eino-tools` from `local-symphony`

> **STRETCH — READ THIS FIRST.** This module has exactly one consumer
> today (`local-symphony`'s worker). The advisor binary is single-shot
> and does not invoke tools; nothing else in the four-repo plan needs
> these tools yet. The boundary cuts below are real refactoring work,
> not a verbatim lift. Recommendation: do not start this extraction
> until at least one of (a) advisor grows a multi-turn / tool-using
> mode, (b) a third Go project enters scope that wants the same tool
> set, or (c) `local-symphony`'s tracker layer ships a second adapter
> and the writer surface stabilises against that second consumer. The
> design notes here are durable in any case — the boundary cuts
> against `dispatcher.ToolOutcome`, `core.IssueState`, and
> `tool.InvokableTool`'s v0.8.x signature change are the same problems
> whenever the extraction lands.

This plan covers extracting four eino-compatible tools — `fileops`,
`search`, `shell`, `trackerwrite` — out of
`/Users/punk1290/git/local-symphony/internal/worker/tools/` into a
standalone Go module at `github.com/mattsp1290/eino-tools` (working
name). Design-only: no Go source, no `go mod init`, no scaffolding.
The output is a sequenced PR list executable without re-deriving the
design.

The four source packages are well-factored — each tool's
`Run(ctx, args) Result` is provider-neutral, the JSON Schema is
inline, and the tools couple to symphony only through three named
imports. The work is reasoning about which couplings travel and
which stay.

## 1. Goals & non-goals

### Goals

- Publish a reusable, opinionated set of eino-compatible
  `compose.Tool` implementations for agentic Go workflows needing
  workspace-scoped file IO, ripgrep-backed search, sandboxed shell
  execution, and an issue-tracker write surface.
- Pin a specific eino minor (`v0.8.13+`); document the supported
  range. Be the canonical home for the JSON tool-result envelope
  (`ToolOutcome` + `ErrorCategory`) the model branches on.
- Preserve the safety properties symphony already enforces:
  workspace containment via post-`EvalSymlinks` descendant checks
  (`fileops/fileops.go:206`, `search/search.go:737`), per-call
  timeouts and process-group SIGKILL on cancel
  (`shell/shell.go:336-356`), output and match caps matching the
  agent's prompt budget.
- Keep test parity intact: ~4,100 lines of existing test code travel
  along, continuing to pass against `t.TempDir()` workspaces with no
  symphony-specific helpers.

### Non-goals

- A generic, LangChain-style toolkit. Scope is bounded to the v1
  ReAct-agent tool set that has shipped. HTTP fetch, notebooks,
  vector-store retrieval — out for v0.x.
- Domain-specific tools (Slack, Jira, GitHub PR creation, Linear).
  Those belong in caller-side or dedicated modules.
- A `TrackerReader` lift. The reader interface
  (`internal/tracker/tracker.go:18-50`) returns `[]core.Issue` and
  `map[string]core.IssueState`, dragging the whole `core` package
  in. It has zero tool consumers — leave it.
- Tracker adapter implementations. The beads adapter
  (`local-symphony/internal/tracker/beads/`) moves to a separate
  `beads-go` module (see
  `~/docs/eino/05-shared-repos-proposal.html#beads-go`).
- The dispatcher event taxonomy (`RunEventKind`, orchestration
  `ErrorCategory`, `RunEvent`). Symphony's orchestration concerns;
  stays in `internal/dispatcher/`. Only `ToolOutcome` — the JSON
  shape the model sees — travels.

### `tracker/` here vs. separate `tracker-go` module

A separate module would let the tracker interface evolve
independently. But today there is one tool consumer
(`trackerwrite`), one adapter (beads), and the writer surface is
four methods. Splitting now creates a two-module dependency for one
consumer with no second pull. Ship under `eino-tools/tracker` for
v0.1.0; revisit when a second tracker or non-trackerwrite consumer
arrives.

## 2. Stretch, not mandatory

Tagged STRETCH for four reasons; weigh each before starting:

- **One consumer.** Advisor is single-shot (no agent loop, no
  tools). Until advisor grows multi-turn mode or a third project
  enters scope, the only beneficiary is symphony — which already
  has these tools in-tree. Extraction adds release ceremony for no
  immediate gain.
- **Boundary cuts are real refactoring.** `dispatcher.ToolOutcome`
  (`internal/dispatcher/event.go:369-379`) is referenced from every
  tool's `Result` envelope and every `IsRetryable`. `core.IssueState`
  (`internal/core/issue.go:48`) is on `trackerwrite.Args.ToState`.
  `cmd.Env = nil` in `shell/shell.go:328` (see section 3) is right
  for symphony's container but is implicit coupling for any other
  consumer. None of these are a verbatim lift.
- **API drift in eino itself.** Symphony pins eino `v0.7.13`
  (`local-symphony/go.mod:6`); the target is `v0.8.13+`. Between
  those minors `tool.InvokableTool` gained `opts ...Option` and
  tools must implement
  `BaseTool.Info(ctx) (*schema.ToolInfo, error)` (`eino@v0.8.13/components/tool/interface.go:42-47`).
  Current symphony tools ship `Schema()` helpers but no `Info()`.
  Non-trivial PR work, not a bullet in a risk register.
- **Tracker interface may evolve.** If a Linear adapter lands in
  symphony first, the writer surface grows (PR linkage, labels)
  better designed once than re-designed. Pull toward extraction is
  strongest after the second tracker.

Recommendation: defer. Reassess when one of the triggers fires.

## 3. Boundary cuts (the real design problem)

Three couplings between the source packages and symphony need
explicit resolution before any code moves.

### 3.1 `dispatcher.ToolOutcome` — the JSON envelope

Every tool's `Result` embeds `dispatcher.ToolOutcome`
(`event.go:372-379`: `succeeded | failed | timed_out | rejected`).
Every `IsRetryable()` branches on the outcome plus a per-tool error
category.

Options:

- **(a) Lift `ToolOutcome` + `ErrorCategory` into
  `eino-tools/result`.** Symphony's `dispatcher` re-exports during
  a transition window; post-cutover dispatcher imports
  `eino-tools/result`. One home for the model-facing JSON shape.
- **(b) Define an `Outcome` interface that `dispatcher.ToolOutcome`
  implements.** Lets dispatcher keep its own type. Costs: every
  `Result` struct switches from typed string to interface, breaking
  `json.Marshal` ergonomics.
- **(c) Hold off until `ToolOutcome` design stabilises.** Equivalent
  to deferring the whole extraction. The four-value taxonomy has
  been stable.

**Recommend (a).** The enum has been stable since v1; the JSON
shape is what the model sees and is the most natural shared
concept. `ToolOutcomeTimedOut` and `ToolOutcomeRejected` are unused
by the current tools (they only emit `Succeeded` / `Failed`) but
lift verbatim — the validator can produce `Rejected` and the
agent's per-turn timeout produces `TimedOut`.

### 3.2 `core.IssueState` — the trackerwrite-only enum

`trackerwrite.Args.ToState` is typed `core.IssueState`
(`trackerwrite.go:78`), a typed string (`core/issue.go:48`)
modelled as open-set. `TrackerWriter.Transition` takes one
(`tracker/tracker.go:65`).

Options:

- **(a) Lift into `eino-tools/tracker.IssueState`.** No constants
  travel (open-set). Symphony's `core.IssueState` becomes a type
  alias for the transition.
- **(b) Generics: `TrackerWriter[S ~string]`.** Decouples the
  interface. Costs: every callsite generic, `Args.ToState` harder
  to expose via JSON Schema, callers bridging trackers cross type
  parameters.

**Recommend (a).** Cost of (b) outstrips the flex gain; the
open-set string is already the open-set choice. Only `IssueState`
travels — `core.Issue`, `core.Priority`, etc. stay (no tool
references them).

### 3.3 `shell.cmd.Env = nil` — implicit env inheritance

**Research correction.** Earlier research at
`~/docs/eino/02-shared-eino-patterns.html#tools` said the shell
tool "hardcoded sandbox proxy env vars (`HTTP_PROXY`, etc.)." It
does not. `shell/shell.go:328` sets `cmd.Env = nil`, inheriting
`os.Environ()`. The comment there explains intent: "explicit per
the spec's 'inherits container env' requirement (HTTP_PROXY /
HTTPS_PROXY for sandbox=default routing through squid)." Symphony
sets the proxy vars on the worker process (container image / squid
sidecar in `internal/workspace/`); the tool inherits them. The tool
contains zero proxy logic. The brief inherited this misclaim; this
is the canonical correction. The boundary cut remains, but its
justification changes.

Actual coupling: a standalone consumer has no way to know it must
populate proxy vars in its own process env, and no way to *override*
the parent env if it wants a curated subset. `os.Environ()`
inheritance is fine inside a symphony container; it is a footgun
elsewhere.

Cut: take an `ExecEnv` struct at construction.

```go
// shell.ExecEnv configures the child process environment.
// Zero value inherits os.Environ(), preserving today's behaviour
// for symphony.
type ExecEnv struct {
    // Env, when non-nil, replaces os.Environ() for the child.
    // Use append(os.Environ(), "HTTP_PROXY=...") for additive.
    Env []string

    // WorkingDir overrides the tool's workspacePath as cmd.Dir.
    // Most consumers should leave this empty.
    WorkingDir string

    // Timeout, when non-zero, overrides DefaultTimeoutSeconds for
    // calls that omit args.timeout_seconds.
    Timeout time.Duration
}

func New(workspacePath string, env ExecEnv) (*Tool, error)
```

Symphony's wiring becomes `shell.New(workspacePath, shell.ExecEnv{})`
— the zero value preserves today's inherit-from-parent semantics.
Standalone consumers declare what they want explicitly.

`fileops` does not need an `ExecEnv` (no child processes). `search`
shells out to `rg` but does not populate `cmd.Env`; egress is
local-disk-only, no proxy routing needed.

## 4. Module layout

Under `github.com/mattsp1290/eino-tools` (path finalised at
`go mod init` in PR1):

```
eino-tools/
  go.mod
  go.sum
  LICENSE                  (TODO; see section 14)
  README.md                (consumer-facing; lifted from the doc.go content)
  result/
    outcome.go             (ToolOutcome enum + IsRetryable helpers)
    error.go               (ErrorCategory, ResultError, classifier helpers)
    outcome_test.go
    golden/                (golden JSON files pinning the wire shape)
  fileops/
    doc.go
    fileops.go             (shared validation helpers + BaseResult)
    read.go                (NewReadTool, ReadArgs, ReadResult, ReadSchema)
    write.go               (NewWriteTool, WriteArgs, WriteResult, WriteSchema)
    edit.go                (NewEditTool, EditArgs, EditResult, EditSchema)
    list.go                (NewListTool, ListArgs, ListResult, ListSchema)
    fileops_test.go
  search/
    doc.go
    search.go              (Tool, New, parseStream, buildMatch, ...)
    search_test.go
    export_test.go
  shell/
    doc.go
    shell.go               (Tool, New, ExecEnv, cappedBuffer, ...)
    shell_test.go
  tracker/
    doc.go
    tracker.go             (TrackerWriter interface, IssueState type)
    errors.go              (Category enum, Error type, CategoryOf, IsRetryable)
    errors_test.go
  trackerwrite/
    doc.go
    trackerwrite.go        (Tool, New, Args, Result, mapCategory, ...)
    trackerwrite_test.go
```

`result/` is the canonical JSON envelope home. `tracker/` exports
the writer interface plus error taxonomy; `trackerwrite/` is the
tool that consumes it. Two packages on purpose: a custom-adapter
consumer imports `tracker/` without pulling the tool. JSON Schemas
stay inline per tool (no shared registry).

## 5. Public API surface

Sketch (parameter names and field ordering pinned to today's symphony
source; field tags and JSON shapes preserved verbatim where possible).

### `result/`

```go
package result

// ToolOutcome is the model-facing outcome enum, JSON-encoded as a
// lowercase string. Lifted verbatim from
// local-symphony/internal/dispatcher/event.go:372-379.
type ToolOutcome string

const (
    ToolOutcomeSucceeded ToolOutcome = "succeeded"
    ToolOutcomeFailed    ToolOutcome = "failed"
    ToolOutcomeTimedOut  ToolOutcome = "timed_out"
    ToolOutcomeRejected  ToolOutcome = "rejected"
)

func (o ToolOutcome) Valid() bool

// ErrorCategory is the cross-tool taxonomy. Per-tool packages
// declare their own const set (e.g. fileops.ErrCategoryPathEscape)
// using plain strings that conform to this type. The category set
// is intentionally not a closed enum here — each tool's error space
// is distinct enough that a shared closed enum would be either too
// broad or too narrow.
type ErrorCategory = string

// ResultError is the standard failure envelope. Per-tool Result
// types embed (or alias) this shape. Optional Op is used by
// trackerwrite; other tools omit it.
type ResultError struct {
    Category ErrorCategory `json:"category"`
    Message  string        `json:"message"`
    Op       string        `json:"op,omitempty"`
}
```

A golden-file test in `result/golden/` pins the JSON shape of
`{"outcome":"failed","error":{"category":"path_escape","message":"..."}}`
so a careless field-tag change cannot silently break the model's
parser. See section 12.

### `fileops/`

```go
func NewReadTool(workspacePath string) (*ReadTool, error)
func NewWriteTool(workspacePath string) (*WriteTool, error)
func NewEditTool(workspacePath string) (*EditTool, error)
func NewListTool(workspacePath string) (*ListTool, error)

// Each *Tool has:
//   Info(ctx) (*schema.ToolInfo, error)   // eino v0.8.x BaseTool
//   InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)
//   Run(ctx, args XxxArgs) XxxResult       // typed entry, lifted as-is
//   XxxSchema() json.RawMessage            // raw schema for advanced wiring
```

### `search/`

```go
func New(workspacePath string) (*Tool, error)

// Tool has Info / InvokableRun / Run / Schema as above.
```

### `shell/`

```go
type ExecEnv struct {
    Env        []string
    WorkingDir string
    Timeout    time.Duration
}

func New(workspacePath string, env ExecEnv) (*Tool, error)
```

The build tag `//go:build unix` on `shell/shell.go:1` stays.
Windows support is out of scope for v0.x; an `errors.ErrUnsupported`
stub on non-unix is acceptable but not required.

### `tracker/`

```go
type IssueState string

type TrackerWriter interface {
    Comment(ctx context.Context, id, body string) error
    Transition(ctx context.Context, id string, toState IssueState) error
    Close(ctx context.Context, id, reason string) error
    LinkPR(ctx context.Context, id, prURL string) error
}

// Category, Error, CategoryOf, IsRetryable all lift verbatim from
// local-symphony/internal/tracker/errors.go.
type Category string

const (
    CategoryOK            Category = "ok"
    CategoryUnknown       Category = ""
    CategoryAPIRequest    Category = "api_request"
    // ... (full set lifted from errors.go)
)

type Error struct {
    Category Category
    Op       string
    ID       string
    Err      error
}
```

### `trackerwrite/`

```go
func New(w tracker.TrackerWriter) (*Tool, error)
```

`Op`, `Args`, `Result`, `ResultError`, `IsRetryable` lift unchanged
modulo the `core.IssueState` → `tracker.IssueState` rename on
`Args.ToState`.

## 6. Tracker sub-package design

The trickiest layout piece: `TrackerWriter` is a stable enough
abstraction to be its own concern, but small enough that an
over-engineered split is wasted.

**Decision:** `eino-tools/tracker` exports the interface plus
canonical errors (`Category`, `Error`, `CategoryOf`, `IsRetryable`).
`eino-tools/trackerwrite` is the tool consuming the interface.

Rationale:

- A custom-adapter consumer imports `tracker/` only.
- A tool consumer imports both. Dep graph:
  `trackerwrite/ → tracker/ → (none)`.
- A future `tracker-go` extraction is a straightforward
  external-module move — this decision does not foreclose it.

The beads adapter does not ship here. It stays in symphony for now,
moves to `beads-go` later per the proposal. `trackerwrite.New` takes
a `tracker.TrackerWriter` at construction.

`TrackerReader` stays in symphony (zero tool consumers; would drag
in `core.Issue` and friends).

## 7. What stays in each consumer

### Symphony (post-migration)

- `internal/worker/tools/` empty (or thin re-export shims during
  the deprecation window; directory deletes one stable release
  later).
- `internal/tracker/` keeps `TrackerReader`, its adapter code, and
  the rate-limit snapshot. `TrackerWriter` re-exports from
  `eino-tools/tracker` for one release, then direct import.
- `internal/dispatcher/event.go` re-exports `ToolOutcome` from
  `eino-tools/result`. `RunEventKind`, orchestration
  `ErrorCategory`, `RunEvent` stay — they are symphony's
  persistence contract, not a model-facing shape.
- `core/issue.go:48` (`IssueState`) becomes a type alias to
  `tracker.IssueState`.
- Beads adapter moves to `beads-go` in a follow-up.

### Advisor

Single-shot today; nothing changes at extraction. When it grows
multi-turn mode, imports `eino-tools/<pkg>` selectively.

### A hypothetical third consumer

The motivating win: a third project imports the module with no
symphony dependency and gets the same hardened semantics symphony's
worker enjoys.

## 8. Eino version pin

Pin `github.com/cloudwego/eino v0.8.13` as the minimum. README
documents that `v0.7.x` is unsupported due to the `InvokableTool`
signature change.

At `v0.8.13`
(`eino@v0.8.13/components/tool/interface.go:42-47`) `InvokableTool`
is:

```go
InvokableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (string, error)
```

and `BaseTool` requires `Info(ctx) (*schema.ToolInfo, error)`.

Symphony's current tools (`fileops/fileops.go:471-484`) assert only
the no-opts shape and ship `ReadSchema()` helpers but no `Info()` —
they do not satisfy v0.8.x `BaseTool`. PR3 must add `opts ...Option`
(impls can ignore the slice for v0.1.0) and synthesise `Info(ctx)`
from the existing inline schema strings. A small helper builds
`*schema.ToolInfo` from name, description, and `json.RawMessage`.

CI matrix tests latest-patch of each supported minor. Symphony's
Phase 0 bump (eino `v0.7.13 → v0.8.13`, Go `1.25.0 → 1.25.5`) is a
prerequisite for consumption.

## 9. Migration plan (symphony)

Ordered to minimise the duration of any "two homes for the same
type" state. Each step is a separate PR in symphony unless noted.

**Step a — Symphony Phase 0 bumps.** Pre-extraction.
`local-symphony/go.mod:6` to eino `v0.8.13`, Go `1.25.5`. Add
`opts ...Option` to each tool's `InvokableRun`. Add `Info(ctx)`
methods. Run worker tests. Symphony-internal — can land before
`eino-tools` exists. If symphony cannot bump (downstream pin),
extraction is blocked here.

**Step b — Lift `ToolOutcome` into `eino-tools/result`.**
`internal/dispatcher/event.go:369-379` becomes:

```go
import "github.com/mattsp1290/eino-tools/result"

type ToolOutcome = result.ToolOutcome

const ToolOutcomeSucceeded = result.ToolOutcomeSucceeded
// ...
```

Type aliases keep callsites unchanged. Dispatcher still owns
`RunEventKind`, orchestration `ErrorCategory`, `RunEvent`.

**Step c — Lift `core.IssueState` and `TrackerWriter`.**
`core/issue.go:48` becomes `type IssueState = tracker.IssueState`.
`internal/tracker/tracker.go:58-76` becomes
`type TrackerWriter = tracker.TrackerWriter`. `Category` and
`Error` similarly. Beads adapter needs no code change — interface
identity is preserved.

**Step d — Replace `internal/worker/tools/` with shims.** Each
package becomes a thin re-export. For `shell`:

```go
package shell

import einoshell "github.com/mattsp1290/eino-tools/shell"

func New(workspacePath string) (*einoshell.Tool, error) {
    return einoshell.New(workspacePath, einoshell.ExecEnv{})
}
```

Zero-value `ExecEnv` matches today's `cmd.Env = nil` semantics.
Symphony's `HTTP_PROXY` routing keeps working — vars are already
on the worker process. Tests move with the source.

**Step e — Delete shims.** One stable release later, delete
`internal/worker/tools/` and update import paths directly. Type
aliases in `dispatcher`/`core` can stay longer at no cost.

## 10. Versioning & release plan

- **v0.1.0.** Four tools lifted with the three boundary cuts
  applied. Eino at `v0.8.13`. Test parity with symphony. No SDK
  guarantees within v0.x.
- **v0.2.0.** Reserve for new tools (`httpfetch`, `notebook`) and
  boundary refinements from a second consumer.
- **v1.0.0.** Tag after at least three months of consumption by
  two-plus distinct callers (symphony + 1). v1 freezes the public
  API and JSON envelope; breaking changes go on `v2/`.

Eino minor is pinned, not floated; bumps that change
`tool.InvokableTool` are explicit `eino-tools` minor bumps.
License: undecided (section 14). Apache-2.0 is the default;
clean-room decision before PR1.

## 11. Test parity

Four test suites totalling 4,098 lines travel verbatim:

- `fileops/fileops_test.go` — 1,065 lines.
- `search/search_test.go` — 1,115 lines (plus `export_test.go` for
  the `defaultRgBinary` seam).
- `shell/shell_test.go` — 1,047 lines.
- `trackerwrite/trackerwrite_test.go` — 871 lines.

No shared helpers under `internal/worker/tools/` — each test file
is self-contained and uses `t.TempDir()` with `os.MkdirAll` /
`os.WriteFile`. Trackerwrite uses an inline `fakeWriter` (no beads
dependency). Nothing else to lift.

Renames on lift: `shell_test.go:154` references
`SYMPHONY_SHELL_INHERIT_PROBE`; rename to
`EINO_TOOLS_SHELL_INHERIT_PROBE` to avoid leaking the prefix. Test
logic otherwise verbatim.

Drift point: tests importing `internal/dispatcher` for
`ToolOutcomeSucceeded` rewrite to `result.ToolOutcomeSucceeded`.
Pre-step-b symphony, dispatcher re-exports cover this.

Add golden-file tests in `result/golden/` per tool (success + one
failure per category). The model parses these envelopes; a careless
field-tag change must be visible at review. Current source has no
such tests — quality-bar addition driven by extraction.

## 12. Risk register

- **JSON shape regression.** Pin the contract via golden tests in
  `result/golden/`. Symphony's type alias means a JSON-shape change
  fans out at compile time, but field-tag drift is silent — golden
  tests catch it.
- **Sandbox-wiring divergence.** A consumer constructs `ExecEnv`
  with permissive defaults and leaks. Document zero-value semantics
  in `shell/doc.go`: "zero value inherits the parent process env;
  in a hardened sandbox, set `Env: []string{}` to start empty."
  Note `nil → inherit` vs `[]string{} → empty` in `exec.Cmd`.
- **Eino API drift.** PR3 handles `v0.7.13 → v0.8.13`. Future
  minors may rev again. Mitigation: CI matrix on multiple supported
  minors; compile-time interface assertions in `_test.go` surface
  drift as a test failure.
- **`TrackerWriter` ossification.** The most-likely-to-evolve
  surface. If a Linear or Jira adapter lands with PR-linkage or
  label-sync needs, the four-method interface may grow. Mitigation:
  optional sidecar interfaces
  (`TrackerWriterLabels { SetLabels(ctx, id, []string) error }`)
  that callers type-assert for, not widening the base.
- **Lack of consumer pull.** If no third project arrives, the
  module stays v0.x and maintenance exceeds the integration win.
  This is why extraction is STRETCH. Plan owner rechecks
  quarterly; pause if no pull materialises.
- **macOS symlink resolution.** `fileops/fileops.go:417-435`
  contains a `canonicalizeWorkspace` helper because macOS routes
  `/var → /private/var`; without it every `/var/folders/...`
  workspace fails containment. CI must include a macOS runner.

## 13. First-PR breakdown

Each PR is reviewable in isolation; no flag day.

**PR1 — Skeleton.** `go.mod`, `LICENSE`, `README.md`,
`.github/workflows/ci.yml` (linux + macOS, latest two Go minors).
No Go source. ~150 lines YAML + Markdown.

**PR2 — `result/`.** `ToolOutcome` + variants + `Valid()` +
`ResultError`. Golden-file tests for the JSON envelope. ~300 lines.

**PR3 — `fileops/`, `search/`, `shell/`.** Apply boundary cuts:
`result.ToolOutcome` everywhere `dispatcher.ToolOutcome` was; add
`ExecEnv` to `shell.New`; add `Info(ctx)` from existing schema
JSON; add `opts ...Option` to `InvokableRun`. Lift tests with the
env-var rename. Largest PR — roughly 5,000 lines (1,500 source +
3,200 tests). Split 3a/3b/3c if review bandwidth limited.

**PR4 — `tracker/` + `trackerwrite/`.** Writer interface,
`Category`, `Error`, `IssueState`. Trackerwrite tool with the
`core.IssueState → tracker.IssueState` rename. Inline `fakeWriter`
test suite. ~1,200 lines.

**PR5 — Tag `v0.1.0` + migrate symphony.** Parallel: tag in
`eino-tools`; in `local-symphony`, do steps b/c/d (section 9) —
add the dep, rewrite the four packages as shims, re-export
`ToolOutcome` and `IssueState`. Step e (delete shims) after the
next stable symphony release.

Cycle time for an attentive engineer: ~2 weeks if symphony Phase 0
is already done, ~4 weeks if not. Phase 0 bump is the dominant
unknown.

## 14. Open questions / TODOs for human review

- **License.** Apache-2.0 vs MIT. Apache-2.0 is the safer default
  for a transitively-depended package. Decision before PR1.
- **`tracker/` here vs separate `tracker-go` module.** Recommend
  here for v0.1.0 (one consumer, small surface). Revisit on second
  tracker / non-trackerwrite consumer of `TrackerWriter`.
- **Beads adapter packaging.** Deferred to a separate `beads-go`
  module. Alternative: a `tracker/beads/` sub-package here for one
  cycle, then move. The sub-package is convenient for migration
  but bakes coupling the proposal wanted to avoid. Default: do not
  ship beads here.
- **`shell.ExecEnv` scope.** Today three fields (`Env`,
  `WorkingDir`, `Timeout`). Should it support `Uid`, `Gid`,
  cgroup hooks, `SysProcAttr` passthrough, pid-namespace flags?
  Default: ship three-field; let v0.2 add fields when a second
  consumer asks.
- **JSON Schema / CUE publication.** Currently only Go types +
  inline JSON Schema strings. Publish CUE / JSON Schema in
  `schemas/` for non-Go consumers? Default: no for v0.1.0.
- **Windows support for `shell`.** `//go:build unix` today. v1.0
  may need an `errors.ErrUnsupported` stub for cross-platform
  builds. Decide before v1.
- **Naming.** Module path `github.com/mattsp1290/eino-tools` is
  working assumption. If the four-repo proposal lands under an org
  namespace, the path changes. Decide before PR1.

---

Plan lives at
`/Users/punk1290/git/eino-tools/docs/prompts/extract-eino-tools.md`;
next step is checking whether section 2's STRETCH triggers have
fired — if not, defer; if yes, schedule PR1 after symphony's Phase
0 eino bump lands.
