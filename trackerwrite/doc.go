// Package trackerwrite implements the tracker_write Eino tool.
//
// The model-facing tool is a discriminated union over comment, transition,
// close, and link_pr operations. V0.1 always executes close through
// tracker.CloseWriter and executes transition when the configured writer also
// satisfies tracker.TransitionWriter. Unsupported operations return a
// structured unsupported_op result so existing prompts receive an actionable
// response.
//
// Writer error mapping is intentionally coarse: only context deadline and
// cancellation map to the timeout/canceled categories; every other writer error
// (including a permanently invalid transition target state) surfaces as the
// retryable unknown category. Writers are therefore responsible for not
// returning permanent failures in a way the agent loop will retry indefinitely;
// a richer category mapping is deferred until a real TransitionWriter exists.
package trackerwrite
