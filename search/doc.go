// Package search implements the agent's ripgrep-backed search tool.
//
// The tool runs ripgrep with the workspace root as cwd, parses ripgrep's NDJSON
// stream, and returns structured matches to the model. The default invocation
// preserves the original regex search behavior; optional args add ripgrep glob
// filters, fixed-string mode, ignore-case mode, context lines, and match caps.
// Callers must provide the rg binary at runtime.
//
// Callers that run concurrent sessions must serialize fileops, glob, search,
// and apply_patch calls per workspace root to avoid validate-then-use
// containment races.
//
// Result preserves the original decoded JSON object in RawJSON so consumers can
// inspect unknown top-level fields added by future versions. RawJSON is never
// emitted when marshaling results produced by this package.
package search
