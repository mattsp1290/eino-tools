// Package result contains shared result primitives used by eino-tools tool
// packages.
//
// Outcome is the stable model-facing discriminator shared by per-tool JSON
// result envelopes. Tool packages keep their own result structs, error
// categories, and retry policies; this package intentionally does not define a
// universal result envelope.
//
// The v0.1 outcome set is closed for validation. Adding another outcome value
// is a breaking API change for consumers that switch exhaustively over tool
// outcomes and must be documented in the changelog.
package result
