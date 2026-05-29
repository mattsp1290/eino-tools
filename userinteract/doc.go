// Package userinteract provides an Eino-compatible tool for asking the user a
// question and collecting their typed answer.
//
// The tool operates in two surfaces:
//
//   - SurfaceCLI: prints the question to stderr and blocks reading a multi-line
//     answer from stdin (terminated by a blank line or EOF). Safe for interactive
//     terminal sessions.
//
//   - SurfaceMCP: returns immediately with OutcomePending and never reads from
//     stdin. The caller (MCP host) is responsible for displaying the question,
//     collecting the user's answer out of band, and supplying it in a follow-up
//     call with Args.Answer populated.
//
// The Surface is set at construction and does not change per call. This matches
// the shell.Tool pattern (workspace path set at New) and structurally prevents an
// MCP-surface instance from ever holding or reading a stdin Reader.
//
// # Outcome type
//
// userinteract defines its own Outcome type (succeeded, pending, failed) rather
// than using result.Outcome. The "pending" state is specific to this tool's MCP
// surface contract; it is not a generic concept and does not belong in the shared
// result package (see ADR 0006).
//
// # IsRetryable
//
// IsRetryable returns false for all outcomes, including unknown. This diverges
// from shell and urlfetch (which return true for unknown). The divergence is
// intentional: stdin I/O errors and validation errors on a human-interaction tool
// are not transient failures that benefit from automated retry.
//
// # Single-outstanding-question assumption
//
// In MCP mode the tool performs no pairing validation between a pending question
// and a subsequent answer. If the agent loop ever has more than one question in
// flight simultaneously, it is the loop's responsibility to route answers
// correctly. The tool cannot help with correlation.
//
// # stdin line cap
//
// In CLI mode the scanner buffer is sized to 1 MiB per line. Pasted input with
// a line longer than 1 MiB is returned as an io error.
package userinteract
