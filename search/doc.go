// Package search implements the agent's ripgrep-backed search tool.
//
// The tool runs `rg --json -e <pattern> -- <path>` with the workspace root as
// cwd, parses ripgrep's NDJSON stream, and returns structured matches to the
// model. Callers must provide the rg binary at runtime.
//
// Result preserves the original decoded JSON object in RawJSON so consumers can
// inspect unknown top-level fields added by future versions. RawJSON is never
// emitted when marshaling results produced by this package.
package search
