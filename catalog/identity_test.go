//go:build unix

package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"

	"github.com/mattsp1290/eino-tools/fileops"
)

func TestValidateDefinitionsRejectsInvalidRecords(t *testing.T) {
	definitions, err := Standard(Options{})
	if err != nil {
		t.Fatal(err)
	}
	base := definitions[0]
	tests := [][]Definition{
		{{}},
		{func() Definition { value := base; value.Binding = BindingKind("bad"); return value }()},
		{func() Definition { value := base; value.ID = ""; return value }()},
		{func() Definition { value := base; value.Name = ""; return value }()},
		{func() Definition { value := base; value.SchemaHash = "INVALID"; return value }()},
		{func() Definition { value := base; value.ExecutorHash = "INVALID"; return value }()},
		{func() Definition { value := base; value.Info = nil; return value }()},
		{func() Definition { value := base; value.New = nil; return value }()},
		{func() Definition { value := base; value.Name = "metadata_mismatch"; return value }()},
		{func() Definition { value := base; value.SchemaHash = strings.Repeat("a", 64); return value }()},
		{base, base},
		{base, func() Definition { value := base; value.ID = "another-id"; return value }()},
	}
	for index, invalid := range tests {
		if err := validateDefinitions(invalid); err == nil {
			t.Errorf("invalid definition %d accepted", index)
		}
	}
}

func TestIdentityGoldens(t *testing.T) {
	definitions, err := Standard(Options{TrackerWriter: &closeWriter{}})
	if err != nil {
		t.Fatal(err)
	}
	schemaGoldens := map[string]string{
		IDFileRead:     "414e371f9333874fbdb2ee927e30717b771561d2caa3900447dc3f34b3254591",
		IDFileWrite:    "2e565e28a9d5c5ef292fae556d213bc4fcdcce234ebe8d727b34a0fdfa47e009",
		IDFileEdit:     "c0c780d90a2dce6c785e50fe13206fe5a6022857488f713480cb73a7676b5bbc",
		IDFileList:     "b10bb60d53ed6692e33f7e3a0eccca64d9691c4965827dc17de0a7d41013d9e6",
		IDGlob:         "114cf917361f0872071618a4de79afbfc4973cdf7d8c816123bb99d422839635",
		IDSearch:       "b68a144ee434ade6866f0f204a65990117504fd54716ece1a18a6956533842b3",
		IDApplyPatch:   "cb9b7cef0b209523c4642f1a4e960bde35bdb209e4bc41d3f5363753b48e54b2",
		IDShell:        "0d4ee75a2b725d44eac76bb35a8719cf5d85e5aa84e73c65ccd2816b1d0f5aa9",
		IDURLFetch:     "3677cf93d326c3a61ec402aafe85694a6ae080f98be555207c7bdea9d603f082",
		IDUserInteract: "f16c14010138ab04a9dc3e57be1bfaf266ba0adc12bac66f2da6a018eb390f2f",
		IDTrackerWrite: "3192a21e8cbad3d50ba253ac6a4387497bae6683a2e6e8126bd8556cd58f86af",
	}
	executorGoldens := map[string]string{
		IDFileRead:     "a0000cfdc19773589bde68fb10ccfbf819352ae3dfbbe5deef016911f8545ce1",
		IDFileWrite:    "6eb94cd8d59fcabeeb281cc41cbba8f484cd0a6785f8f0a9946c5e722eab19cd",
		IDFileEdit:     "e55b3e76d9625b2689b771eb3bc4549ca4df864280bdc0a4fa19d5a69dfcf7d1",
		IDFileList:     "e6b88764b09e13c071d6b46926c00e95ae738fad82c49aa5ccc21ea6c7ad723f",
		IDGlob:         "492b6bc7c1ac0476deca74ab4d36a3d450ae1cb755c28ce82e6c1af4619c50a8",
		IDApplyPatch:   "631020234ac45f41f9fcef93858f676a3baf2979edfef7d3dfc489de95d45faa",
		IDURLFetch:     "3785a268f524bf79622092c8e72b736fe421bb3b302384dc8721ff6bcf8bdb8f",
		IDUserInteract: "871b7ceb5f94d3e5a1ddc949d61f373dd5e8a65f6083e3e43196a7dee443ea85",
		IDTrackerWrite: "2a33d7b717a9803115950b9da94e9e0b12d2153b66e9e312d930883bad93f2ee",
	}
	for _, definition := range definitions {
		if want := schemaGoldens[definition.ID]; definition.SchemaHash != want {
			t.Errorf("%s schema hash = %s, want %s", definition.ID, definition.SchemaHash, want)
		}
		// Search and shell identities intentionally include the host executable
		// path/content plus captured environment, so their exact value is a
		// deployment snapshot rather than a repository-wide golden.
		if want, ok := executorGoldens[definition.ID]; ok && definition.ExecutorHash != want {
			t.Errorf("%s executor hash = %s, want %s", definition.ID, definition.ExecutorHash, want)
		}
	}
}

func TestExecutorHashGolden(t *testing.T) {
	dependencies := []executorDependency{
		{Kind: "z-target", Path: "/synthetic/z", ContentSHA256: strings.Repeat("b", 64)},
		{Kind: "a-invocation", Path: "/synthetic/a", ContentSHA256: strings.Repeat("a", 64)},
	}
	got, err := executorHash("synthetic.tool", 7, dependencies, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	const want = "b2457573d1dbd91e20ff4f67058aef4f2198132b7d95ecd92d669ab45d5a7f08"
	if got != want {
		t.Fatalf("executorHash = %s, want %s", got, want)
	}
}

func TestMakeDefinitionRejectsInvalidSpecs(t *testing.T) {
	newTool := func(context.Context, Instance) (tool.InvokableTool, error) { return nil, nil }
	base := definitionSpec{
		id: IDFileRead, revision: 1, name: fileops.NameRead, binding: BindingWorkspace,
		info: fileops.ReadToolInfo, newTool: newTool,
	}
	tests := []definitionSpec{
		func() definitionSpec { spec := base; spec.revision = 0; return spec }(),
		func() definitionSpec { spec := base; spec.info = nil; return spec }(),
		func() definitionSpec { spec := base; spec.newTool = nil; return spec }(),
	}
	for index, spec := range tests {
		if definition, err := makeDefinition(spec); err == nil || definition.New != nil {
			t.Errorf("invalid spec %d = %+v, %v; want zero definition and error", index, definition, err)
		}
	}
}
