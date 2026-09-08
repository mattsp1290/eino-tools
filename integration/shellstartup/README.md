# Shell startup mounted acceptance

This separate test module uses published `eino-agent v0.3.3` and requires Go
>=1.26.3. The root library requires Go 1.26; startup mode adds no production
dependency. The local replacement exercises the current checkout, and is test
wiring only. Consumers should use the published implementation commit supplied
in the implementation response, with no replacement.

The compiling mount fixture in `startup_test.go` exercises this public nesting:

```go
mounted, err := einotools.MountStandard(ctx, registry, component, einotools.Options{
    Scope: extension.GlobalScope(),
    Catalog: catalog.Options{
        ShellOptions: &shell.Options{
            ShellBinary: "/bin/sh",
            StartupMode: shell.StartupModeNonLogin,
            Env: allowlistedEnv,
        },
    },
})
```

The host owns `ctx`, registry, component artifact identity, admitted workspace,
and `allowlistedEnv`. The complete catalog requires a real discoverable ripgrep,
even when the run exposes only shell. Tests resolve an absolute ripgrep path.
Every acquired plan must be released before closing its mount.

Run from this directory:

```sh
GOWORK=off go mod tidy -diff
GOWORK=off go vet ./...
GOWORK=off go test -race -v -count=1 -timeout=120s ./...
```

Linux and macOS CI run real `/bin/sh -c` fixtures. Shared
`internal/shelltest` fixtures verify synthetic HOME profile exclusion and a
same-file explicit-source positive control, command syntax, stdin EOF,
noninteractive flags, both stream caps, environment ownership, workspace cwd,
and timeout/cancellation of non-detached descendants. Mounted tests also compare
persisted tool identities with identical component artifact/config identities,
reject invalid mount configuration atomically, and verify that extra model JSON
fields cannot change host policy. The published adapter currently discards
unknown fields; a rejecting decoder is accepted too.

Only the disposable login CI container sets `EINO_TOOLS_TEST_LOGIN=1`.
**Never set this on a developer host.** That container has inert system startup
files and only synthetic per-test HOME directories. It adds real login variants
and an automatic `.profile` positive control, then repeats non-login exclusion.
Ordinary host tests prove exact default/login flags with an argv recorder.

For final publication verification, copy this module outside the checkout,
remove the local replacement, fetch the pushed full implementation SHA, and
repeat tidy, vet and race tests. Verify `go list -m -json` has no replacement.
The response records the resulting pseudo-version and platform evidence.
