// Package jsoncompat contains small JSON compatibility helpers shared by tool
// packages.
package jsoncompat

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RejectDuplicateTopLevelKeys rejects duplicate keys in a top-level JSON
// object. Malformed JSON is ignored here so callers can return their normal
// unmarshal error.
func RejectDuplicateTopLevelKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil //nolint:nilerr // defer to Unmarshal for the canonical error
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil
	}
	seen := make(map[string]struct{})
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil //nolint:nilerr // defer to Unmarshal for the canonical error
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil
		}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate top-level key %q", key)
		}
		seen[key] = struct{}{}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil //nolint:nilerr // defer to Unmarshal for the canonical error
		}
	}
	return nil
}
