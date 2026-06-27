// Package glob implements a workspace-rooted file discovery tool.
//
// Patterns use github.com/bmatcuk/doublestar/v4 semantics: *, ?, character
// classes, and ** for recursive directory matching. The implementation walks
// the workspace directly, includes hidden paths by default, and skips VCS
// internals such as .git, .hg, and .svn. Ignore-file support is intentionally
// deferred; callers that need .gitignore semantics should use search with
// ripgrep glob filters or add that policy at the runtime layer.
//
// The containment contract matches the other filesystem tools in this module:
// callers must serialize filesystem tool calls per workspace root to avoid
// symlink-swap races between path validation and use.
package glob
