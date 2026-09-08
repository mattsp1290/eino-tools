//go:build unix

package shell_test

import (
	"github.com/mattsp1290/eino-tools/shell"
)

func ExampleNew() {
	// The host supplies an admitted absolute workspace and allowlisted environment.
	workspace := "/srv/workspace"
	allowlistedEnv := []string{"PATH=/usr/bin:/bin", "HOME=/srv/tool-home"}
	tool, err := shell.New(workspace, shell.Options{
		ShellBinary: "/bin/sh",
		StartupMode: shell.StartupModeNonLogin,
		Env:         allowlistedEnv,
	})
	if err != nil {
		panic(err)
	}
	_ = tool // Pass to the host's tool runner.
}
