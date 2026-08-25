//go:build unix

package catalog_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/applypatch"
	"github.com/mattsp1290/eino-tools/fileops"
	"github.com/mattsp1290/eino-tools/glob"
	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
	"github.com/mattsp1290/eino-tools/trackerwrite"
	"github.com/mattsp1290/eino-tools/urlfetch"
	"github.com/mattsp1290/eino-tools/userinteract"
)

type infoSource interface {
	Info(context.Context) (*schema.ToolInfo, error)
}

type metadataCloser struct{}

func (metadataCloser) Close(context.Context, string, string) error { return nil }

func TestLeafMetadataBuildersMatchReceiverAndSchema(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		builder   func() (*schema.ToolInfo, error)
		schema    func() json.RawMessage
		construct func() (infoSource, error)
	}{
		{"file read", fileops.ReadToolInfo, fileops.ReadSchema, func() (infoSource, error) { return fileops.NewReadTool(root) }},
		{"file write", fileops.WriteToolInfo, fileops.WriteSchema, func() (infoSource, error) { return fileops.NewWriteTool(root) }},
		{"file edit", fileops.EditToolInfo, fileops.EditSchema, func() (infoSource, error) { return fileops.NewEditTool(root) }},
		{"file list", fileops.ListToolInfo, fileops.ListSchema, func() (infoSource, error) { return fileops.NewListTool(root) }},
		{"glob", glob.ToolInfo, glob.Schema, func() (infoSource, error) { return glob.New(root) }},
		{"search", search.ToolInfo, search.Schema, func() (infoSource, error) { return search.New(root) }},
		{"apply patch", applypatch.ToolInfo, applypatch.Schema, func() (infoSource, error) { return applypatch.New(root) }},
		{"shell", shell.ToolInfo, shell.Schema, func() (infoSource, error) { return shell.New(root) }},
		{"URL fetch", urlfetch.ToolInfo, urlfetch.Schema, func() (infoSource, error) { return urlfetch.New() }},
		{"user interaction", userinteract.ToolInfo, userinteract.Schema, func() (infoSource, error) { return userinteract.New(userinteract.SurfaceMCP) }},
		{"tracker write", trackerwrite.ToolInfo, trackerwrite.Schema, func() (infoSource, error) { return trackerwrite.New(metadataCloser{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first, err := test.builder()
			if err != nil {
				t.Fatal(err)
			}
			second, err := test.builder()
			if err != nil {
				t.Fatal(err)
			}
			if first == second || first.ParamsOneOf == second.ParamsOneOf {
				t.Fatal("builder did not return fresh metadata")
			}
			originalName := second.Name
			first.Name = "mutated"
			first.Desc = "mutated"
			firstSchema, err := first.ToJSONSchema()
			if err != nil {
				t.Fatal(err)
			}
			firstSchema.Description = "mutated"
			third, err := test.builder()
			if err != nil {
				t.Fatal(err)
			}
			if third.Name != originalName {
				t.Fatalf("metadata mutation leaked: %q", third.Name)
			}
			thirdSchema, err := third.ToJSONSchema()
			if err != nil {
				t.Fatal(err)
			}
			if third.Desc == "mutated" || thirdSchema.Description == "mutated" {
				t.Fatal("description or schema mutation leaked")
			}

			leaf, err := test.construct()
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := leaf.Info(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if receiver.Name != second.Name || receiver.Desc != second.Desc {
				t.Fatalf("receiver metadata differs from builder")
			}
			builderSchema, err := second.ToJSONSchema()
			if err != nil {
				t.Fatal(err)
			}
			receiverSchema, err := receiver.ToJSONSchema()
			if err != nil {
				t.Fatal(err)
			}
			var exportedSchema any
			if err := json.Unmarshal(test.schema(), &exportedSchema); err != nil {
				t.Fatal(err)
			}
			builderJSON, _ := json.Marshal(builderSchema)
			var normalizedBuilder any
			if err := json.Unmarshal(builderJSON, &normalizedBuilder); err != nil {
				t.Fatal(err)
			}
			receiverJSON, _ := json.Marshal(receiverSchema)
			var normalizedReceiver any
			if err := json.Unmarshal(receiverJSON, &normalizedReceiver); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalizedBuilder, exportedSchema) || !reflect.DeepEqual(normalizedReceiver, exportedSchema) {
				t.Fatal("builder, receiver, and exported schema differ")
			}
		})
	}
}
