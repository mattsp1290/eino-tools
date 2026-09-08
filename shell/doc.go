// Package shell provides an Eino-compatible shell execution tool.
//
// The package intentionally exposes the execution boundary as configuration:
// callers own workspace containment, sandboxing, and environment policy.
//
// Options.StartupMode defaults to StartupModeLogin (sh -lc). Explicit
// StartupModeNonLogin selects -c; command text remains one unchanged argument.
// Both modes are noninteractive. Configured executables must support these flags.
// Non-login /bin/sh avoids login profiles; other shells can read startup files
// even with -c (for example Bash named bash reads BASH_ENV). Hosts own environment
// scrubbing and executable provenance. Explicit Env replaces the environment;
// nil inherits at execution in the leaf, or is captured at catalog construction.
// Process-group cancellation cleans up the created group, not detached processes.
//
// Result preserves the original decoded JSON object in RawJSON so consumers can
// inspect unknown top-level fields added by future versions. RawJSON is never
// emitted when marshaling results produced by this package.
package shell
