package search

import "fmt"

// Options configures ripgrep execution. Zero values preserve the historical
// direct-package behavior: resolve "rg" at invocation, inherit the process
// environment, and allow ripgrep's user configuration.
type Options struct {
	RGBinary      string
	Env           []string
	DisableConfig bool
}

func resolveOptions(opts []Options) (Options, error) {
	switch len(opts) {
	case 0:
		return Options{RGBinary: defaultRgBinary}, nil
	case 1:
		o := opts[0]
		if o.RGBinary == "" {
			o.RGBinary = defaultRgBinary
		}
		if o.Env != nil {
			o.Env = append([]string(nil), o.Env...)
		}
		return o, nil
	default:
		return Options{}, fmt.Errorf("search: expected at most one Options value, got %d", len(opts))
	}
}
