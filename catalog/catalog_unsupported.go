//go:build !unix

package catalog

import "fmt"

// Standard returns the standard catalog. Full execution is Unix-only because
// the shell leaf is Unix-only.
func Standard(_ Options) ([]Definition, error) {
	return nil, fmt.Errorf("%w", ErrUnsupportedPlatform)
}
