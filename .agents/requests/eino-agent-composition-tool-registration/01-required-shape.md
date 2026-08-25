# Required provider shape

Expose a deterministic, runtime-neutral description of the standard leaf-tool
set that a consumer can translate into its own composition records. Exact names
are open to implementation, but the shape should have these properties.

```go
type ScopeKind string

const (
	ScopeStatic    ScopeKind = "static"
	ScopeWorkspace ScopeKind = "workspace"
)

type Definition struct {
	ID             string
	Name           string
	Scope          ScopeKind
	RetrySafe      bool
	Concurrent     bool
	SchemaHash     string
	ExecutorHash   string
	New            func(context.Context, Instance) (tool.InvokableTool, error)
}

type Instance struct {
	WorkspaceRoot string
}

func Standard(options Options) ([]Definition, error)
```

This is illustrative, not a required spelling. A successful design must:

- return definitions in deterministic order with stable, unique IDs;
- expose enough information to obtain `ToolInfo` without binding a real
  workspace or using `/` as a sentinel root;
- distinguish workspace-bound from static factories;
- state whether calls may run concurrently for one canonical workspace root;
- provide restart-stable schema and executor identity, or canonical bytes from
  which the consumer can compute those hashes;
- preserve injectable surfaces such as shell options, URL-fetch HTTP policy,
  user interaction, and tracker writers;
- create a fresh or immutable execution object per materialization; and
- make factory failure atomic from the caller's perspective.

The consumer will translate each returned definition into an
`eino-agent/composition.ToolRegistration`, attach the host-owned artifact and
config identity, and mount the resulting component. The returned leaf object
must remain ordinary Eino tooling; it must not know about run plans, sessions,
permission approval, or persistence.

## Identity requirements

`ID` identifies the registration within the standard bundle and must not be
constructed by concatenating other fields with a magic delimiter. `Name` is the
model-visible Eino tool name. They are separate identity dimensions.

`SchemaHash` changes whenever the model-visible name, description, or parameter
schema changes. `ExecutorHash` changes whenever execution semantics change in a
way that could make a resumed call differ. If this repository cannot publish
those hashes directly, it should publish deterministic canonical identity
inputs and document the hashing algorithm.

