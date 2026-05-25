// Package fileops provides workspace-rooted file operation tools for Eino.
//
// All paths accepted by this package are workspace-relative. The package
// validates path shape, resolves symlinks, and rejects operations that escape
// the configured workspace root.
//
// The current containment model assumes one tool call executes at a time for a
// workspace. If callers introduce concurrent tool execution against the same
// workspace, the resolver layer needs openat-style containment around a held
// workspace file descriptor to close symlink-swap races.
package fileops
