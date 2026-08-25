//go:build !unix

package catalog

import (
	"errors"
	"testing"
)

func TestStandardUnsupportedPlatform(t *testing.T) {
	definitions, err := Standard(Options{})
	if definitions != nil || !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("Standard = %#v, %v; want nil ErrUnsupportedPlatform", definitions, err)
	}
}
