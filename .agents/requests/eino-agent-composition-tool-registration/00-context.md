# Eino-agent composition tool registration

Request owner: `eino-agent`

Tracking issue: `eino-tools-uzx`

## Why this request exists

`eino-agent` consumes the leaf tools in this repository, but its current
`tools/einotools.RegisterDefaults` adapter registers them into an isolated
`tools.Registry`. The production runtime no longer reads that registry. It
acquires an immutable `runtime.RunPlan` from a `composition.Registry`, seals
tool behavior together with restart-stable identity, persists the resulting
descriptor, and recreates the same plan before resume.

The old registration helper therefore succeeds without making any of the
standard tools executable. There are no released users of either API, so this
request intentionally asks for the correct boundary rather than a compatibility
shim.

## Eino-agent in one page

For each run, `eino-agent`:

1. snapshots mounted components for a global or session scope;
2. selects enabled tool capabilities;
3. freezes each definition and its executable provenance into a run plan;
4. persists a deterministic descriptor fingerprint;
5. materializes tools from bounded session/workspace data; and
6. on resume, reacquires the exact identities and rejects drift before work.

The runtime needs more than an Eino `InvokableTool`. A mounted tool also has a
stable registration ID, artifact/config/executor identity, scope resolution,
permission requirements, retry safety, output-retention policy, and a factory
that can bind a concrete workspace without consulting mutable global state.

## Ownership boundary

`eino-tools` should continue to own leaf schemas and execution. `eino-agent`
should continue to own durable composition, permissions, session identity,
workspace admission, and run-plan persistence. The requested API must not make
`eino-tools` import `eino-agent`.

