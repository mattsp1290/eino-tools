// Package shell provides an Eino-compatible shell execution tool.
//
// The package intentionally exposes the execution boundary as configuration:
// callers own workspace containment, sandboxing, and environment policy.
//
// Result preserves the original decoded JSON object in RawJSON so consumers can
// inspect unknown top-level fields added by future versions. RawJSON is never
// emitted when marshaling results produced by this package.
package shell
