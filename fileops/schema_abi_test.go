package fileops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFileopsSchemaABI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     json.RawMessage
		properties []string
		required   []string
	}{
		{name: NameRead, schema: ReadSchema(), properties: []string{"path"}, required: []string{"path"}},
		{name: NameWrite, schema: WriteSchema(), properties: []string{"path", "content", "create_dirs"}, required: []string{"path", "content"}},
		{name: NameEdit, schema: EditSchema(), properties: []string{"path", "anchor", "replacement"}, required: []string{"path", "anchor", "replacement"}},
		{name: NameList, schema: ListSchema(), properties: []string{"path", "recursive"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got struct {
				Type                 string                     `json:"type"`
				AdditionalProperties bool                       `json:"additionalProperties"`
				Properties           map[string]json.RawMessage `json:"properties"`
				Required             []string                   `json:"required"`
			}
			if err := json.Unmarshal(tt.schema, &got); err != nil {
				t.Fatalf("schema parse: %v", err)
			}
			if got.Type != "object" {
				t.Fatalf("type = %q, want object", got.Type)
			}
			if got.AdditionalProperties {
				t.Fatal("additionalProperties = true, want false")
			}
			for _, property := range tt.properties {
				if _, ok := got.Properties[property]; !ok {
					t.Fatalf("property %q missing from %#v", property, got.Properties)
				}
			}
			for _, required := range tt.required {
				if !slices.Contains(got.Required, required) {
					t.Fatalf("required %q missing from %#v", required, got.Required)
				}
			}
		})
	}
}

func TestFileopsResultJSONABI(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "read.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "edit.txt"), []byte("hello anchor"), 0o600); err != nil {
		t.Fatalf("write edit fixture: %v", err)
	}

	read := mustNewReadTool(t, workspace)
	write := mustNewWriteTool(t, workspace)
	edit := mustNewEditTool(t, workspace)
	list := mustNewListTool(t, workspace)

	assertJSONFields(t, read.Run(context.Background(), ReadArgs{Path: "read.txt"}),
		"outcome", "path", "content", "content_bytes")
	assertJSONFields(t, write.Run(context.Background(), WriteArgs{Path: "write.txt", Content: "created"}),
		"outcome", "path", "bytes_written", "created")
	assertJSONFields(t, edit.Run(context.Background(), EditArgs{Path: "edit.txt", Anchor: "anchor", Replacement: "replacement"}),
		"outcome", "path", "bytes_written", "anchor_occurrences")
	assertJSONFields(t, list.Run(context.Background(), ListArgs{Path: "."}),
		"outcome", "path", "entries")
}

func assertJSONFields(t *testing.T, value any, fields ...string) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	for _, field := range fields {
		if _, ok := got[field]; !ok {
			t.Fatalf("field %q missing from %s", field, raw)
		}
	}
}
