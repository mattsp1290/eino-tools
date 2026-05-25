# Shell extraction inventory

Source inspected: `/home/infra-admin/git/local-symphony/internal/worker/tools/shell`
on 2026-05-25.

## Files

- `doc.go`: package contract, `sh -lc` rationale, sandbox boundary, stdin
  behavior, and future option notes.
- `shell.go`: tool implementation, schema, timeout/output caps, process-group
  cancellation, result envelope, duplicate-key guard, and retry policy.
- `shell_test.go`: execution, timeout/cancellation, truncation, schema,
  invokable, retry, and process cleanup tests.

## Public Surface To Preserve

- Tool name: `Name = "shell"`.
- Constants:
  - `DefaultTimeoutSeconds = 60`
  - `MaxTimeoutSeconds = 600`
  - `OutputCapBytes = 256 * 1024`
- `Schema() json.RawMessage` returns a fresh copy.
- `New(workspacePath string)` rejects empty and non-absolute workspace paths.
  It does not stat the directory at construction time.
- `Tool.Run(ctx, Args) Result` always returns a structured `Result` for command
  execution outcomes.
- `Tool.Info(ctx)` returns Eino `*schema.ToolInfo`.
- `Tool.InvokableRun(ctx, argsJSON string, opts ...tool.Option) (string, error)`
  rejects empty, malformed, and duplicate top-level JSON before execution.

## Schema Contract

The schema is an object with `additionalProperties: false`.

Required:

- `cmd`

Properties:

- `cmd`: string, `minLength: 1`.
- `timeout_seconds`: integer, `minimum: 0`, `maximum: 600`.

`timeout_seconds` omitted or set to zero uses the 60 second default. Negative
values and values above 600 fail validation at execution and through the JSON
Schema validator.

## Execution Semantics

- Commands run as `sh -lc <cmd>` with `exec.CommandContext`.
- `cmd.Dir` is the configured workspace path.
- `stdin` is an empty reader, so the child process sees immediate EOF.
- `cmd.Env` is nil in production, meaning the process inherits the parent
  environment. Tests can inject an explicit environment slice.
- The Unix implementation sets `SysProcAttr.Setpgid = true` and cancels by
  sending `SIGKILL` to the process group, so backgrounded descendants are killed
  with the shell.
- `cmd.WaitDelay` is five seconds after cancellation.
- A missing workspace path is not caught by `New`; command start fails with an
  `exec_failed` result.

The `sh -lc` behavior is intentional compatibility. Local-symphony documents
that operators rely on login-shell profile setup for credential helpers and
proxy environment. Do not silently switch this to `sh -c`; make it an explicit
option if the extracted package needs a stricter mode.

## Timeout And Cancellation

Timeout handling uses a child context created from the caller context. The
effective command timeout is:

- `DefaultTimeoutSeconds` when `timeout_seconds` is zero or omitted.
- The requested value when it is between 1 and `MaxTimeoutSeconds`.

Result classification order:

1. `cmd.Run()` returns nil -> `outcome=succeeded`, `exit_code=0`.
2. Parent context is canceled or expired -> `outcome=failed`,
   `error.category=canceled`.
3. Tool-created timeout expires while the parent context is still active ->
   `outcome=failed`, `error.category=timeout`, `timed_out=true`.
4. `*exec.ExitError` -> `outcome=succeeded` with the command's nonzero exit
   code.
5. Any other run error -> `outcome=failed`, `error.category=exec_failed`,
   `exit_code=-1`.

This means a nonzero shell exit is a successful tool invocation. The model must
inspect `exit_code` rather than treating nonzero as a failed tool result.

## Output Caps

`stdout` and `stderr` are capped independently at `OutputCapBytes` bytes. The
tool returns UTF-8-aligned strings and sets `stdout_truncated` or
`stderr_truncated` when bytes beyond the cap were observed.

## Result Envelope

`Result` fields:

- `Outcome`: replace `dispatcher.ToolOutcome` with `result.Outcome`.
- `ExitCode`
- `Stdout`
- `Stderr`
- `StdoutTruncated`
- `StderrTruncated`
- `DurationMS`
- `TimedOut`
- `Error`

`ResultError` fields:

- `Category`
- `Message`

Error category strings to preserve:

- `validation`
- `timeout`
- `canceled`
- `exec_failed`
- `unknown`

Retryable result categories:

- `timeout`
- `unknown`

Successful invocations, including nonzero shell exits, are not retryable.

## Sandbox Boundary

The shell tool only sets the working directory and process execution behavior.
Workspace containment, filesystem/network sandboxing, secrets policy, and
permission boundaries are owned by the caller/container. The extracted package
should expose constructor options for policy choices without implying stronger
containment than it enforces.

## Coupling To Replace

- Replace `local-symphony/internal/dispatcher.ToolOutcome*` with
  `eino-tools/result.Outcome*`.
- Keep Eino imports against `github.com/cloudwego/eino/components/tool` and
  `github.com/cloudwego/eino/schema`.
- Keep `github.com/eino-contrib/jsonschema` for schema-to-ToolInfo conversion
  unless a shared helper replaces it.
- Do not import local-symphony dispatcher, telemetry, auth, core, or worker
  packages.

## Tests To Lift

The implementation work should lift or recreate tests for:

- Successful command execution, stdout/stderr capture, duration, and `exit_code`.
- Nonzero shell exit returning `outcome=succeeded`.
- Default timeout, max timeout, negative timeout, and timeout above max.
- Tool timeout versus parent-context cancellation classification.
- Killing backgrounded child processes on cancellation.
- Independent stdout/stderr truncation, UTF-8 cap alignment, and zero-length
  writes after reaching the cap.
- Missing workspace and missing shell binary `exec_failed` behavior.
- Invokable JSON serialization, empty input, malformed input, and duplicate
  top-level key rejection.
- Schema validity, fresh schema slices, required `cmd`, unknown properties,
  empty `cmd`, timeout bounds, and non-string `cmd`.
- Stable `Name`, success envelope omission of optional error/timeout fields,
  retry policy, nil receiver behavior, and concurrency safety.
