package fileops

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestFileopsResultsPreserveUnknownFieldsInRawJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
		out  any
	}{
		{
			name: "read",
			raw:  []byte(`{"outcome":"succeeded","path":"a.txt","content":"a","future":"read"}`),
			out:  &ReadResult{},
		},
		{
			name: "write",
			raw:  []byte(`{"outcome":"succeeded","path":"a.txt","bytes_written":1,"future":"write"}`),
			out:  &WriteResult{},
		},
		{
			name: "edit",
			raw:  []byte(`{"outcome":"succeeded","path":"a.txt","bytes_written":1,"anchor_occurrences":1,"future":"edit"}`),
			out:  &EditResult{},
		},
		{
			name: "list",
			raw:  []byte(`{"outcome":"succeeded","path":".","entries":[],"future":"list"}`),
			out:  &ListResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := json.Unmarshal(tt.raw, tt.out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			var raw []byte
			switch got := tt.out.(type) {
			case *ReadResult:
				raw = got.RawJSON
			case *WriteResult:
				raw = got.RawJSON
			case *EditResult:
				raw = got.RawJSON
			case *ListResult:
				raw = got.RawJSON
			default:
				t.Fatalf("unknown result type %T", tt.out)
			}
			if !bytes.Contains(raw, []byte(`"future"`)) {
				t.Fatalf("RawJSON = %s, want future field preserved", raw)
			}

			encoded, err := json.Marshal(tt.out)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if bytes.Contains(encoded, []byte("RawJSON")) || bytes.Contains(encoded, []byte("future")) {
				t.Fatalf("marshal emitted raw/future field: %s", encoded)
			}
		})
	}
}
