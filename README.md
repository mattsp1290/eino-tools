# eino-tools

Standalone Go/Eino tool packages extracted from `local-symphony` for reuse by
agent workers.

## Packages

- `result`: shared model-facing outcome enum.
- `fileops`: workspace-rooted `file_read`, `file_write`, `file_edit`, and
  `file_list` tools. `file_read` also supports line-windowed reads.
- `glob`: doublestar-backed workspace path discovery tool.
- `search`: ripgrep-backed workspace search tool with regex/literal modes,
  glob filters, ignore-case, context lines, and match limits.
- `applypatch`: `apply_patch` tool for multi-file add/update/delete/move
  patches.
- `shell`: shell command execution tool.
- `tracker`: tracker interfaces used by tracker tools.
- `tracker/beads`: adapter from `github.com/mattsp1290/beads-go/beads` to
  `tracker.CloseWriter`.
- `trackerwrite`: `tracker_write` tool; v0.1 executes `op=close` through
  `tracker.CloseWriter`, optionally executes `op=transition` through
  `tracker.TransitionWriter`, and optionally executes `op=comment` through
  `tracker.CommentWriter`.
- `urlfetch`: `url_fetch` tool; fetches raw text from `file://` or `https://` URLs.
- `userinteract`: `user_interact` tool; asks the user a question and returns their
  answer; works in CLI (blocking stdin) and MCP (non-blocking pending/answer) modes.
- `catalog`: deterministic metadata, identity, safety declarations, and fresh
  factories for the standard leaf-tool set.

## Requirements

- Go 1.26 or newer.
- `rg` on `PATH` for `search`.
- `bd` on `PATH` only for consumers that use `tracker/beads` with the default
  beads-go exec-backed client, or for the tagged integration test.

## Development

Run the standard local gates from the repository root:

```sh
make test
make vet
make lint
```

`make lint` is the supported lint entry point. It pins `golangci-lint` and runs
it with a Go toolchain compatible with the repo's Go 1.26 lint target. A
`golangci-lint run` binary from `PATH` may fail before linting if that binary was
built with an older Go version.

## Examples

```go
ok := result.OutcomeSucceeded
if !ok.Valid() {
	panic("unexpected outcome")
}
```

```go
read, err := fileops.NewReadTool(workspace)
if err != nil {
	return err
}
res := read.Run(ctx, fileops.ReadArgs{Path: "README.md"})
```

```go
offset := 40
limit := 80
res := read.Run(ctx, fileops.ReadArgs{
	Path:   "README.md",
	Offset: &offset,
	Limit:  &limit,
})
```

```go
globTool, err := glob.New(workspace)
if err != nil {
	return err
}
res := globTool.Run(ctx, glob.Args{Pattern: "**/*_test.go", Limit: 500})
```

```go
searchTool, err := search.New(workspace)
if err != nil {
	return err
}
res := searchTool.Run(ctx, search.Args{
	Pattern:    "func New",
	Path:       "search",
	Glob:       search.Globs{"**/*.go"},
	IgnoreCase: true,
	Context:    2,
})
```

```go
patchTool, err := applypatch.New(workspace)
if err != nil {
	return err
}
res := patchTool.Run(ctx, applypatch.Args{PatchText: patchText})
```

```go
shellTool, err := shell.New(workspace, shell.Options{
	Env:            []string{"PATH=/usr/bin:/bin"},
	ShellBinary:    "sh",
	OutputCapBytes: shell.OutputCapBytes,
})
if err != nil {
	return err
}
res := shellTool.Run(ctx, shell.Args{Cmd: "go test ./...", TimeoutSeconds: 60})
```

```go
definitions, err := catalog.Standard(catalog.Options{})
if err != nil {
	return err
}
info, err := definitions[0].Info() // no workspace or fake root required
if err != nil {
	return err
}

// The host admits and canonicalizes the root, then supplies it only when the
// frozen registration is materialized.
leaf, err := definitions[0].New(ctx, catalog.Instance{WorkspaceRoot: canonicalRoot})
_ = info
_ = leaf
```

```go
type closer struct{}

func (closer) Close(ctx context.Context, id, reason string) error {
	return nil
}

trackerTool, err := trackerwrite.New(closer{})
if err != nil {
	return err
}
res := trackerTool.Run(ctx, trackerwrite.Args{
	Op:     trackerwrite.OpClose,
	ID:     "project-123",
	Reason: "fixed",
})
```

```go
beadsTracker, err := beads.NewClient(
	beadssdk.WithDataDir(".beads"),
	beadssdk.WithActor("agent"),
)
if err != nil {
	return err
}
err = beadsTracker.Close(ctx, "project-123", "fixed")
```

## Runtime Notes

`search` shells out to `rg`; consumers must provide ripgrep on `PATH`.

Filesystem tools (`fileops`, `glob`, `search`, and `applypatch`) currently use
validate-then-use path containment. Consumers that run multiple tool calls
against the same workspace must serialize those filesystem calls per workspace
root. Independent workspace roots may run concurrently. A future openat-style
implementation can relax this contract.

The standard `catalog` marks every workspace definition `Concurrent=false`, so
the host must use one shared keyed lock for all catalog workspace tools bound to
the same canonical root; the catalog does not provide that lock. Leaf binding
is separate from host global/session mount scope. A static binding also does not
mean permission-free: `url_fetch` can access HTTPS and absolute `file://` paths.

The full standard catalog executes on Unix. On non-Unix platforms its public
types remain available, while `catalog.Standard` returns
`catalog.ErrUnsupportedPlatform` and no definitions.

`shell` intentionally runs model-provided commands with host-configured policy
(default `sh -lc <cmd>`, or explicit non-login `-c`) in the
configured workspace. The package sets cwd, stdin, output caps, timeout, and
process cancellation behavior; filesystem containment, network policy, secrets,
and sandboxing are caller responsibilities.

`tracker/beads` uses the beads-go SDK. It does not import `os/exec` directly and
does not parse `bd` CLI output itself; command execution and JSON decoding live
in `beads-go`.

All tools return model-facing JSON envelopes with stable `outcome` strings.
Result structs that may be decoded by consumers preserve unknown top-level JSON
fields in `RawJSON` for forward compatibility.

`web_search` and model-facing LSP/diagnostics tools are intentionally out of
scope for this module. `eino-agent` should own those schemas and runtime
policies because they require credentials, freshness policy, server lifecycle,
indexing state, cancellation, and permission handling.

## Consumers

The first target consumers are
[`local-symphony`](https://github.com/mattsp1290/local-symphony), which will use
thin wrappers around these packages during adoption, and
[`eino-agent`](https://github.com/mattsp1290/eino-agent), which consumes the
coding-agent leaf tools.

### Explicit shell startup policy

Hosts can select non-login, noninteractive execution through the existing API:

```go
shell.New(workspace, shell.Options{
    ShellBinary: "/bin/sh",
    StartupMode: shell.StartupModeNonLogin,
    Env: allowlistedEnv,
})
```

The host supplies the admitted absolute workspace and allowlisted environment.
Empty mode and `StartupModeLogin` preserve `sh -lc`; `StartupModeNonLogin`
selects `-c`. Commands remain one unchanged argument. Unknown modes and negative
output caps fail construction. Mode, binary and environment remain host options,
not model schema properties. See the [compiling leaf example](shell/example_test.go)
and [eino-agent mount example](integration/shellstartup/README.md).

A nonnil Env replaces the environment, including an empty slice. Nil inherits
at each leaf execution; the standard catalog snapshots inheritance at creation.
Catalog policy and environment are copied; shell executor revision 2 fingerprints
normalized mode and effective cap. Rebuild affected persisted plans under the new
identity instead of bypassing drift checks.

Configured binaries must accept the selected flags. Linux/macOS CI verify
`/bin/sh -c` against synthetic HOME profiles, with isolated login regression CI.
Non-login mode is not a sandbox: Bash invoked as `bash` may still read `BASH_ENV`.
Hosts own environment scrubbing, executable provenance and OS containment.
Cancellation kills the created process group, not deliberately detached processes.
Root requires Go 1.26; the mount example uses eino-agent v0.3.3 and Go >=1.26.3.
The complete catalog needs ripgrep even if only shell is exposed.
