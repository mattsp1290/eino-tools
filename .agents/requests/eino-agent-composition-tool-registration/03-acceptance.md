# Acceptance criteria

- A test can enumerate the standard bundle twice and obtain the same ordered
  IDs, names, schema identities, executor identities, and scope declarations.
- Every current standard tool is represented: file read/write/edit/list, glob,
  search, apply-patch, shell, URL fetch, user interaction, and configured
  tracker write.
- Metadata inspection does not construct a tool against a fake root.
- Materializing the same workspace definition for two different roots produces
  isolated tools with no later lookup through mutable global registration.
- Optional dependencies and constructor failures follow the explicit behavior
  in `02-lifecycle-and-errors.md`.
- The package has no import of `github.com/mattsp1290/eino-agent`.
- Package documentation includes a short adapter example showing how a host
  maps one definition into its own registry.
- `make test`, `make vet`, and `make lint` pass.

## Non-goals

- Implementing `eino-agent` composition or persistence in this repository.
- Preserving the existing disconnected registration helper.
- Adding feature flags, migrations, deprecation aliases, or generation APIs.
- Moving session-native tools such as plan, retained-output, subagent, or skill
  loading into `eino-tools`; those remain host/runtime capabilities.

