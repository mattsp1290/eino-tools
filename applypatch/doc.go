// Package applypatch implements the model-facing apply_patch tool.
//
// The grammar is the Codex-style patch envelope:
//
//	*** Begin Patch
//	*** Add File: <path>
//	+<line>
//	*** Update File: <path>
//	*** Move to: <new-path>
//	@@
//	 <context>
//	-<removed>
//	+<added>
//	*** Delete File: <path>
//	*** End Patch
//
// CRLF and CR patch input are normalized to LF for parsing. Text written by add
// hunks uses LF. Updates preserve the existing file's CRLF line endings when a
// CRLF file is patched. Patch lines are line-oriented and written with a final
// newline; no special EOF-no-newline marker is supported in this version.
//
// The containment contract matches the other filesystem tools in this module:
// callers must serialize filesystem tool calls per workspace root to avoid
// symlink-swap races between path validation and use.
package applypatch
