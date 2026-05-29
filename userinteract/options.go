package userinteract

import (
	"io"
	"os"
)

// Options configures userinteract tool behavior that is intentionally owned
// by the caller rather than hidden inside the tool.
type Options struct {
	// Stdin overrides the reader used for CLI input. Default: os.Stdin.
	Stdin io.Reader
	// Stderr overrides the writer used to print the question prompt. Default:
	// os.Stderr.
	Stderr io.Writer
}

func (o Options) withDefaults() Options {
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	return o
}
