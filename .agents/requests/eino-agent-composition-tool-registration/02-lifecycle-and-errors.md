# Lifecycle and error contract

The catalog itself is configuration, not an owner of live resources. Each
factory invocation either returns a fully usable leaf tool or an error. The
consumer may retain a returned tool for the lifetime of one frozen run plan.

Workspace tools receive a host-canonicalized workspace root. They must not
recapture process cwd or a later mutable option. Tools that require shared
serialization should declare that fact; `eino-agent` will supply the keyed
locker because workspace authority belongs to the host.

Optional tools should be omitted only because an explicit dependency was not
configured, such as a nil tracker writer. Constructor errors for configured
tools must be returned, not converted into silent omission. Duplicate IDs or
names, missing identity material, invalid schemas, and nil factories are
definition errors.

The API should not expose register/replace/unregister generations. Dynamic
mount lifetime is owned by `eino-agent`'s composition registry, and the run plan
already freezes the exact generation selected for a run.

No compatibility layer is required for the disconnected
`tools/einotools.RegisterDefaults` helper. Once the new provider surface is
available and consumed, `eino-agent` will delete that helper and its private
registry path.

