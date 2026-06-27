// Package fileops provides workspace-rooted file operation tools for Eino.
//
// All paths accepted by this package are workspace-relative. The package
// validates path shape, resolves symlinks, and rejects operations that escape
// the configured workspace root.
//
// file_read preserves legacy {path} prefix reads and adds optional offset/limit
// line-windowed reads with raw and numbered content.
//
// The current containment model assumes one filesystem tool call executes at a
// time for a workspace. Callers that run concurrent sessions must serialize
// fileops, glob, search, and apply_patch calls per workspace root. If callers
// need parallel filesystem mutation/discovery against the same workspace, the
// resolver layer needs openat-style containment around a held workspace file
// descriptor to close symlink-swap races.
package fileops
