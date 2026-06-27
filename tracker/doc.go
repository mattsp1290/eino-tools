// Package tracker defines the small tracker mutation interface used by
// eino-tools.
//
// V0.1 intentionally exposes issue close plus optional string-based transition
// and comment writer capabilities. Reader APIs, PR linkage, and issue-state
// types stay with consumers until a later design promotes them into this shared
// module.
package tracker
