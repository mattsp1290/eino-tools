//go:build unix

package shell_test

import (
	"testing"

	"github.com/mattsp1290/eino-tools/internal/shelltest"
	"github.com/mattsp1290/eino-tools/shell"
)

func TestStartupContract(t *testing.T) {
	shelltest.Contract(t, func(t *testing.T, root string, options *shell.Options) shelltest.Run {
		tool, err := shell.New(root, *options)
		if err != nil {
			t.Fatal(err)
		}
		return tool.Run
	})
}
