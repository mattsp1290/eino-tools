// Package urlfetch provides an Eino-compatible tool for fetching the raw text
// content of a file:// or https:// URL.
//
// Unlike the fileops tools, urlfetch intentionally reaches outside any
// workspace boundary. The canonical use case is loading documentation assets
// from absolute filesystem paths (e.g. ~/docs/learning-output-style/index.html)
// or from HTTPS endpoints. The absence of workspace containment is deliberate;
// reviewers should not flag it as an oversight.
//
// Result preserves the original decoded JSON object in RawJSON so consumers
// can inspect unknown top-level fields added by future versions. RawJSON is
// never emitted when marshaling results produced by this package.
package urlfetch
