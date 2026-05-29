# eino-tools

Standalone Go/Eino tool packages extracted from `local-symphony` for reuse by
agent workers.

## Packages

- `result`: shared model-facing outcome enum.
- `fileops`: workspace-rooted `file_read`, `file_write`, `file_edit`, and
  `file_list` tools.
- `search`: ripgrep-backed workspace search tool.
- `shell`: shell command execution tool.
- `tracker`: tracker interfaces used by tracker tools.
- `tracker/beads`: adapter from `github.com/mattsp1290/beads-go/beads` to
  `tracker.CloseWriter`.
- `trackerwrite`: `tracker_write` tool; v0.1 executes `op=close` through
  `tracker.CloseWriter`.
- `urlfetch`: `url_fetch` tool; fetches raw text from `file://` or `https://` URLs.
- `userinteract`: `user_interact` tool; asks the user a question and returns their
  answer; works in CLI (blocking stdin) and MCP (non-blocking pending/answer) modes.

## Requirements

- Go 1.26 or newer.
- `rg` on `PATH` for `search`.
- `bd` on `PATH` only for consumers that use `tracker/beads` with the default
  beads-go exec-backed client, or for the tagged integration test.

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
searchTool, err := search.New(workspace)
if err != nil {
	return err
}
res := searchTool.Run(ctx, search.Args{Pattern: "func New", Path: "search"})
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

`shell` intentionally runs model-provided commands as `sh -lc <cmd>` in the
configured workspace. The package sets cwd, stdin, output caps, timeout, and
process cancellation behavior; filesystem containment, network policy, secrets,
and sandboxing are caller responsibilities.

`tracker/beads` uses the beads-go SDK. It does not import `os/exec` directly and
does not parse `bd` CLI output itself; command execution and JSON decoding live
in `beads-go`.

All tools return model-facing JSON envelopes with stable `outcome` strings.
Result structs that may be decoded by consumers preserve unknown top-level JSON
fields in `RawJSON` for forward compatibility.

## Consumers

The first target consumer is
[`local-symphony`](https://github.com/mattsp1290/local-symphony), which will use
thin wrappers around these packages during adoption.
