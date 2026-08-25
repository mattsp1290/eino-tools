// Package catalog publishes a deterministic, runtime-neutral description of
// the standard eino-tools leaf set.
//
// BindingWorkspace means a factory requires a host-admitted canonical root;
// BindingStatic means it does not. Binding is not global/session mount scope or
// permission policy. Concurrent false requires the host to serialize calls
// using one keyed lock for a canonical root (or shared static dependency); the
// catalog intentionally owns no locks.
//
// Schema hashes cover model-visible metadata. Executor hashes cover the stable
// registration ID, a manually maintained per-tool revision, and inspectable
// executable/environment provenance. Executable aliases retain their absolute
// invocation path while the resolved target is fingerprinted separately;
// identity inputs must be valid UTF-8. Increment only the affected executor
// revision when execution semantics change; metadata-only changes update schema
// identity instead. Hosts must compose these leaf identities with their own
// artifact, configuration, permission, retention, and mount identities.
package catalog
