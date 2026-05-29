# Tool Spec: `userinteract`

**Package:** `github.com/mattsp1290/eino-tools/userinteract`  
**Status:** Pending engineer sign-off on three design questions (see `00-overview.md` Q1–Q3)  
**Dependencies introduced:** none (all stdlib)

---

## What it does

Asks the user a question and returns their typed answer as a string. Must work correctly in two surfaces:

- **CLI:** print the question to stderr, block reading a multi-line answer from stdin (terminated by a blank line or EOF), return the answer.
- **MCP:** return immediately with a `pending` signal (no blocking) so the MCP server can release its thread. The caller is responsible for collecting the user's answer and providing it in a follow-up tool call.

The tool does not format questions, validate answers, or offer multiple-choice options. The model owns question content; this tool owns I/O plumbing.

---

## Files to create

```
userinteract/
├── doc.go
├── userinteract.go
└── userinteract_test.go
```

`docs/inventory/userinteract.md` and a README entry are also required.

---

## Surface type

```go
// Surface identifies the runtime context in which the tool is operating.
type Surface string

const (
    // SurfaceCLI is for interactive terminal sessions where blocking on stdin is safe.
    SurfaceCLI Surface = "cli"
    // SurfaceMCP is for MCP server contexts where blocking is fatal.
    SurfaceMCP Surface = "mcp"
)
```

Surface is set at construction, not per-call. A given agent loop runs on one surface. This mirrors the shell.Tool idiom (workspace path at construction) and satisfies the request's requirement for an explicit signal rather than auto-detection (auto-detecting via `isatty` is insufficient when MCP also has a terminal attached).

---

## Constructor

```go
type Tool struct {
    surface Surface
    stdin   io.Reader
    stderr  io.Writer
}

func New(surface Surface, opts ...Options) (*Tool, error) { ... }
```

Validate that `surface` is one of `SurfaceCLI` or `SurfaceMCP`.

### Options

```go
type Options struct {
    // Stdin overrides the reader used for CLI input. Default: os.Stdin.
    Stdin io.Reader
    // Stderr overrides the writer used to print the question prompt. Default: os.Stderr.
    Stderr io.Writer
}
```

Injectable streams are **required for testability**, not just convenience. The critical MCP test is that `Run` returns without ever reading from stdin. That test is only possible if a panicking reader can be injected. See test cases below.

---

## Args

```go
type Args struct {
    Question string `json:"question"`
    Answer   string `json:"answer,omitempty"`
}
```

- `question` is required (non-empty string).
- `answer` is optional. When present and non-empty, it takes precedence over the surface — the tool echoes it immediately as a successful result on both surfaces.

### JSON Schema

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "question": {
      "type": "string",
      "minLength": 1,
      "description": "The question to present to the user."
    },
    "answer": {
      "type": "string",
      "description": "When non-empty, the tool returns this as the answer immediately without blocking. Used in MCP mode to provide the user's response in a follow-up call."
    }
  },
  "required": ["question"]
}
```

---

## Three-way call precedence

```
1. answer non-empty  →  return succeeded{answer} immediately (both surfaces)
2. surface == CLI    →  print question to stderr, read answer from stdin, return succeeded{answer}
3. surface == MCP    →  return pending{question} immediately (no stdin read)
```

This is stateless. The tool holds no cross-call state. The MCP contract is:
1. Agent calls `user_interact` with `question`.
2. Tool returns `pending` — the question text is in the result so the MCP host can display it.
3. MCP host collects the user's answer (out of band).
4. Agent calls `user_interact` again with both `question` and `answer`.
5. Tool returns `succeeded` with the answer.

The agent loop (not the tool) is responsible for persisting state between calls 2 and 4.

---

## Result

### Design question (Q1): Outcome representation

**Recommended: tool-local `Outcome` enum**

```go
// Outcome is userinteract's own discriminator. It is not result.Outcome.
// The "pending" outcome only exists in this tool; adding it to the shared
// result package would violate ADR 0001's closed-enum rule.
type Outcome string

const (
    OutcomeSucceeded Outcome = "succeeded" // answer available in Answer field
    OutcomePending   Outcome = "pending"   // MCP mode: answer not yet available
    OutcomeFailed    Outcome = "failed"
)
```

**Why this over `result.Outcome + Pending bool`:** A single switch on `Outcome` in the consumer is cleaner than checking two fields. The tradeoff is deviating from the "all tools use `result.Outcome`" convention — call this out explicitly in `doc.go`.

**Alternative: `result.Outcome` + `Pending bool`**

```go
type Result struct {
    Outcome result.Outcome  `json:"outcome"` // "succeeded" or "failed"
    Pending bool            `json:"pending,omitempty"` // true when MCP awaiting answer
    Answer  string          `json:"answer,omitempty"`
    Error   *ResultError    `json:"error,omitempty"`
    RawJSON json.RawMessage `json:"-"`
}
```

`outcome: "succeeded"` with `pending: true` and no `answer` is semantically awkward — "succeeded at what?" — but it avoids any deviation from the shared enum. Engineers who switch exhaustively on `result.Outcome` won't have a gap.

**Engineer sign-off required on this choice before implementation.**

### Recommended Result struct (using tool-local Outcome)

```go
type Result struct {
    Outcome  Outcome         `json:"outcome"`
    Answer   string          `json:"answer,omitempty"`
    Question string          `json:"question,omitempty"` // echoed in pending results
    Error    *ResultError    `json:"error,omitempty"`
    RawJSON  json.RawMessage `json:"-"`
}
```

`Question` is included in pending results so the MCP host has the text to display to the user without needing to track it separately.

### ResultError

```go
type ResultError struct {
    Category string `json:"category"`
    Message  string `json:"message"`
}
```

### Error categories

| Category | When |
|----------|------|
| `validation` | Empty question, unknown surface value |
| `io` | Stdin read error in CLI mode |
| `unknown` | Unexpected errors |

### IsRetryable

- All failures → `false`. Validation errors won't become valid on retry. IO errors on stdin are not transient in any meaningful sense. Pending is not an error.

---

## CLI mode: stdin reading

```
1. Write "<question>\n> " to t.stderr
2. Read lines from t.stdin in a loop
3. Accumulate lines into answer until:
   a. A blank line is read (terminates answer, blank line not included), OR
   b. EOF
4. Return succeeded{answer: strings.TrimRight(accumulated, "\n")}
```

Use a `bufio.Scanner` on `t.stdin`. Handle EOF via `scanner.Err() == nil && !scanner.Scan()` pattern.

---

## MCP mode: non-blocking return

```
1. Return Result{Outcome: OutcomePending, Question: args.Question}
```

No stdin read, no stderr write. Return immediately.

---

## Test cases

All tests use `t.Parallel()`. The "panicking reader" pattern is the critical MCP test.

```
TestNew
  - SurfaceCLI with defaults succeeds
  - SurfaceMCP with defaults succeeds
  - Unknown surface value returns error
  - Custom stdin/stderr injected correctly

TestTool_Run_AnswerProvided (both surfaces)
  - answer in args → succeeded immediately, no stdin read
  - works on SurfaceCLI
  - works on SurfaceMCP

TestTool_Run_CLI
  - single-line answer (blank line terminator): returns answer without trailing blank
  - multi-line answer (blank line terminator): joined correctly
  - answer terminated by EOF: works
  - question printed to injected stderr (capture and assert)
  - stdin read error → io error result

TestTool_Run_MCP
  - returns OutcomePending immediately
  - Question field echoed in result
  - stdin is NEVER read (inject a reader that panics on Read — if panic fires, test fails)
  - no write to stderr

TestTool_Run_Validation
  - empty question → validation error

TestTool_InvokableRun
  - empty argsJSON → error
  - duplicate keys rejected
  - valid JSON round-trips through Run

TestResult_UnmarshalJSON
  - RawJSON preserved

TestSchema
  - valid JSON returned
  - required fields correct

TestIsRetryable
  - succeeded → false
  - pending → false
  - failed/validation → false
  - failed/io → false
```

The panicking reader test pattern:

```go
type panicReader struct{}

func (panicReader) Read(_ []byte) (int, error) {
    panic("userinteract: Read called in MCP mode — must not block")
}
```

---

## Model-facing tool description (for Info())

> Ask the user a question and return their answer as a string. In CLI mode, prints the question to stderr and reads a multi-line answer from stdin (terminated by a blank line or EOF). In MCP mode, returns immediately with a pending result — provide the user's answer in a follow-up call by populating the `answer` field. Does not format questions, validate answers, or offer multiple-choice options.

---

## Proposed ADR text (for docs/adr/ after engineer confirms)

```
# ADR 00XX: userinteract tool-local Outcome enum

## Status
Proposed.

## Context
userinteract needs to signal three states: succeeded (answer available), pending
(MCP mode, answer not yet collected), and failed. The shared result.Outcome enum
(ADR 0001) is explicitly closed: unknown values fail validation, and adding a new
outcome is a breaking change requiring a minor-version bump.

"pending" is specific to userinteract's MCP surface contract. No other tool will
ever return it. Adding it to result.Outcome would couple a general-purpose enum
to a single tool's I/O protocol.

## Decision
userinteract defines its own Outcome type with three values: succeeded, pending,
failed. It does not import result.Outcome. This deviation from the "all tools use
result.Outcome" convention is noted explicitly in doc.go.

## Consequences
Consumers that want to handle tool results generically by outcome cannot treat
userinteract.Outcome and result.Outcome as the same type. Callers must import the
userinteract package directly. This is acceptable: the tool's surface-aware
pending state is not a generic concept and consumers are expected to handle it
explicitly.
```

---

## New imports required in go.mod

None. All dependencies are in the Go standard library (`bufio`, `io`, `os`).
