// Package trackerwrite implements the tracker_write Eino tool.
//
// The model-facing tool is a discriminated union over comment, transition,
// close, and link_pr operations. V0.1 always executes close through
// tracker.CloseWriter and executes transition when the configured writer also
// satisfies tracker.TransitionWriter. Unsupported operations return a
// structured unsupported_op result so existing prompts receive an actionable
// response.
package trackerwrite
