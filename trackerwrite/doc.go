// Package trackerwrite implements the tracker_write Eino tool.
//
// The model-facing tool is a discriminated union over comment, transition,
// close, and link_pr operations. V0.1 executes only close through
// tracker.CloseWriter; the other operations return a structured
// unsupported_op result so existing prompts receive an actionable response.
package trackerwrite
