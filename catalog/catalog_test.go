//go:build unix

package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
	"github.com/mattsp1290/eino-tools/tracker"
	"github.com/mattsp1290/eino-tools/urlfetch"
	"github.com/mattsp1290/eino-tools/userinteract"
)

type closeWriter struct{ calls int }

func (writer *closeWriter) Close(_ context.Context, _, _ string) error {
	writer.calls++
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

type nilReader struct{}

func (*nilReader) Read([]byte) (int, error) { return 0, io.EOF }

type nilWriter struct{}

func (*nilWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestStandardContract(t *testing.T) {
	first, err := Standard(Options{})
	if err != nil {
		t.Fatalf("Standard first: %v", err)
	}
	second, err := Standard(Options{})
	if err != nil {
		t.Fatalf("Standard second: %v", err)
	}

	type expectedDefinition struct {
		id         string
		name       string
		binding    BindingKind
		retrySafe  bool
		concurrent bool
	}
	expected := []expectedDefinition{
		{IDFileRead, "file_read", BindingWorkspace, true, false},
		{IDFileWrite, "file_write", BindingWorkspace, false, false},
		{IDFileEdit, "file_edit", BindingWorkspace, false, false},
		{IDFileList, "file_list", BindingWorkspace, true, false},
		{IDGlob, "glob", BindingWorkspace, true, false},
		{IDSearch, "search", BindingWorkspace, true, false},
		{IDApplyPatch, "apply_patch", BindingWorkspace, false, false},
		{IDShell, "shell", BindingWorkspace, false, false},
		{IDURLFetch, "url_fetch", BindingStatic, true, true},
		{IDUserInteract, "user_interact", BindingStatic, false, false},
	}
	if len(first) != len(expected) || len(second) != len(expected) {
		t.Fatalf("definition count = %d, %d; want %d", len(first), len(second), len(expected))
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	for index, want := range expected {
		got := first[index]
		if got.ID != want.id || got.Name != want.name || got.Binding != want.binding ||
			got.RetrySafe != want.retrySafe || got.Concurrent != want.concurrent {
			t.Errorf("definition[%d] = %+v; want %+v", index, got, want)
		}
		if ids[got.ID] || names[got.Name] {
			t.Fatalf("duplicate definition identity at %q/%q", got.ID, got.Name)
		}
		ids[got.ID], names[got.Name] = true, true
		if !validHash(got.SchemaHash) || !validHash(got.ExecutorHash) {
			t.Errorf("%s has invalid hashes: schema=%q executor=%q", got.ID, got.SchemaHash, got.ExecutorHash)
		}
		other := second[index]
		if got.ID != other.ID || got.Name != other.Name || got.Binding != other.Binding ||
			got.RetrySafe != other.RetrySafe || got.Concurrent != other.Concurrent ||
			got.SchemaHash != other.SchemaHash || got.ExecutorHash != other.ExecutorHash {
			t.Errorf("definition %s is not stable across enumeration", got.ID)
		}
		info1, err := got.Info()
		if err != nil {
			t.Fatalf("Info %s: %v", got.ID, err)
		}
		info2, err := got.Info()
		if err != nil {
			t.Fatalf("Info second %s: %v", got.ID, err)
		}
		if info1 == info2 || info1.ParamsOneOf == info2.ParamsOneOf || info1.Name != got.Name {
			t.Errorf("%s metadata is not fresh or name mismatched", got.ID)
		}
		if _, err := info1.ToJSONSchema(); err != nil {
			t.Errorf("%s schema conversion: %v", got.ID, err)
		}
	}
}

func TestStandardOptionalTrackerAndFreshFactories(t *testing.T) {
	writer := &closeWriter{}
	definitions, err := Standard(Options{TrackerWriter: writer})
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if len(definitions) != 11 || definitions[10].ID != IDTrackerWrite {
		t.Fatalf("tracker catalog order = %#v", definitionIDs(definitions))
	}
	root := canonicalTempDir(t)
	for _, definition := range definitions {
		instance := Instance{}
		if definition.Binding == BindingWorkspace {
			instance.WorkspaceRoot = root
		}
		first, firstErr := definition.New(context.Background(), instance)
		if firstErr != nil {
			t.Fatalf("first New %s: %v", definition.ID, firstErr)
		}
		second, secondErr := definition.New(context.Background(), instance)
		if secondErr != nil {
			t.Fatalf("second New %s: %v", definition.ID, secondErr)
		}
		if reflect.ValueOf(first).Pointer() == reflect.ValueOf(second).Pointer() {
			t.Errorf("%s factory returned the same tool pointer", definition.ID)
		}
	}
	trackerDef := findDefinition(t, definitions, IDTrackerWrite)
	trackerTool, err := trackerDef.New(context.Background(), Instance{})
	if err != nil {
		t.Fatalf("tracker New: %v", err)
	}
	if _, err := trackerTool.InvokableRun(context.Background(), `{"op":"close","id":"issue-1"}`); err != nil {
		t.Fatalf("tracker invocation: %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("tracker calls = %d, want 1", writer.calls)
	}
}

func TestWorkspaceFactoriesRejectInvalidRoots(t *testing.T) {
	definitions, err := Standard(Options{TrackerWriter: &closeWriter{}})
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	valid := canonicalTempDir(t)
	regular := filepath.Join(valid, "file")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(filepath.Dir(valid), filepath.Base(valid)+"-alias")
	if err := os.Symlink(valid, symlink); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(symlink) })
	removed := canonicalTempDir(t)
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}
	cases := []string{"", "relative", regular, symlink, removed, valid + string(filepath.Separator)}
	for _, definition := range definitions {
		if definition.Binding != BindingWorkspace {
			continue
		}
		for _, root := range cases {
			if got, err := definition.New(context.Background(), Instance{WorkspaceRoot: root}); err == nil || got != nil {
				t.Errorf("%s New(%q) = %T, %v; want nil error", definition.ID, root, got, err)
			}
		}
	}
}

func TestFactoriesHonorCanceledContext(t *testing.T) {
	definitions, err := Standard(Options{TrackerWriter: &closeWriter{}})
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := canonicalTempDir(t)
	for _, definition := range definitions {
		got, err := definition.New(ctx, Instance{WorkspaceRoot: root})
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Errorf("%s canceled New = %T, %v", definition.ID, got, err)
		}
	}
}

func TestWorkspaceRootIsolation(t *testing.T) {
	definitions, err := Standard(Options{})
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	rootA := canonicalTempDir(t)
	rootB := canonicalTempDir(t)
	writeFile(t, rootA, "marker.txt", "alpha_token\n")
	writeFile(t, rootB, "marker.txt", "beta_token\n")
	writeFile(t, rootA, "edit.txt", "alpha\n")
	writeFile(t, rootB, "edit.txt", "beta\n")
	writeFile(t, rootA, "alpha.only", "a")
	writeFile(t, rootB, "beta.only", "b")

	invokePairContains(t, definitions, IDFileRead, rootA, rootB,
		`{"path":"marker.txt"}`, "alpha_token", "beta_token")
	invokePairContains(t, definitions, IDFileList, rootA, rootB,
		`{"path":"."}`, "alpha.only", "beta.only")
	invokePairContains(t, definitions, IDGlob, rootA, rootB,
		`{"pattern":"*.only"}`, "alpha.only", "beta.only")
	invokePairContains(t, definitions, IDSearch, rootA, rootB,
		`{"pattern":"_token","path":"."}`, "alpha_token", "beta_token")
	invokePairContains(t, definitions, IDShell, rootA, rootB,
		`{"cmd":"pwd"}`, rootA, rootB)

	writeDef := findDefinition(t, definitions, IDFileWrite)
	invokeAt(t, writeDef, rootA, `{"path":"written.txt","content":"alpha"}`)
	invokeAt(t, writeDef, rootB, `{"path":"written.txt","content":"beta"}`)
	assertFile(t, rootA, "written.txt", "alpha")
	assertFile(t, rootB, "written.txt", "beta")

	editDef := findDefinition(t, definitions, IDFileEdit)
	invokeAt(t, editDef, rootA, `{"path":"edit.txt","anchor":"alpha","replacement":"edited-alpha"}`)
	invokeAt(t, editDef, rootB, `{"path":"edit.txt","anchor":"beta","replacement":"edited-beta"}`)
	assertFile(t, rootA, "edit.txt", "edited-alpha\n")
	assertFile(t, rootB, "edit.txt", "edited-beta\n")

	patchDef := findDefinition(t, definitions, IDApplyPatch)
	patchA, _ := json.Marshal(map[string]string{"patch_text": "*** Begin Patch\n*** Add File: patched.txt\n+alpha\n*** End Patch\n"})
	patchB, _ := json.Marshal(map[string]string{"patch_text": "*** Begin Patch\n*** Add File: patched.txt\n+beta\n*** End Patch\n"})
	invokeAt(t, patchDef, rootA, string(patchA))
	invokeAt(t, patchDef, rootB, string(patchB))
	assertFile(t, rootA, "patched.txt", "alpha\n")
	assertFile(t, rootB, "patched.txt", "beta\n")
}

func TestCapturedOptionsAndExecutableDrift(t *testing.T) {
	rgPath, lookupErr := exec.LookPath("rg")
	if lookupErr != nil {
		t.Skip("rg unavailable")
	}
	shellOptions := &shell.Options{ShellBinary: "/bin/sh", Env: []string{"PATH=/usr/bin:/bin", "CATALOG_VALUE=one"}}
	searchOptions := &search.Options{RGBinary: rgPath, Env: []string{"PATH=/usr/bin:/bin"}}
	definitions, err := Standard(Options{ShellOptions: shellOptions, SearchOptions: searchOptions})
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	shellOptions.Env[1] = "CATALOG_VALUE=two"
	shellOptions.ShellBinary = "/missing"
	searchOptions.RGBinary = "/missing"
	t.Setenv("PATH", "/definitely-not-the-captured-path")
	t.Setenv("CATALOG_VALUE", "process-two")
	root := canonicalTempDir(t)
	shellDef := findDefinition(t, definitions, IDShell)
	output := invokeAt(t, shellDef, root, `{"cmd":"printf %s \"$CATALOG_VALUE\""}`)
	if !strings.Contains(output, "one") || strings.Contains(output, "two") {
		t.Fatalf("shell did not retain captured options: %s", output)
	}
	if _, materializeErr := findDefinition(t, definitions, IDSearch).New(context.Background(), Instance{WorkspaceRoot: root}); materializeErr != nil {
		t.Fatalf("search did not retain captured options: %v", materializeErr)
	}

	temp := canonicalTempDir(t)
	fake := filepath.Join(temp, "fake-rg")
	if writeErr := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o700); writeErr != nil {
		t.Fatal(writeErr)
	}
	drifted, err := Standard(Options{
		SearchOptions: &search.Options{RGBinary: fake, Env: []string{"PATH=/usr/bin:/bin"}},
		ShellOptions:  &shell.Options{ShellBinary: "/bin/sh", Env: []string{"PATH=/usr/bin:/bin"}},
	})
	if err != nil {
		t.Fatalf("Standard with fake rg: %v", err)
	}
	if writeErr := os.WriteFile(fake, []byte("#!/bin/sh\nexit 1\n"), 0o700); writeErr != nil {
		t.Fatal(writeErr)
	}
	got, err := findDefinition(t, drifted, IDSearch).New(context.Background(), Instance{WorkspaceRoot: root})
	if err == nil || got != nil || !strings.Contains(err.Error(), "identity drift") {
		t.Fatalf("drift New = %T, %v", got, err)
	}
	updated, err := Standard(Options{
		SearchOptions: &search.Options{RGBinary: fake, Env: []string{"PATH=/usr/bin:/bin"}},
		ShellOptions:  &shell.Options{ShellBinary: "/bin/sh", Env: []string{"PATH=/usr/bin:/bin"}},
	})
	if err != nil {
		t.Fatalf("Standard after executable replacement: %v", err)
	}
	before := findDefinition(t, drifted, IDSearch)
	after := findDefinition(t, updated, IDSearch)
	if before.ExecutorHash == after.ExecutorHash || before.SchemaHash != after.SchemaHash {
		t.Fatalf("search identity did not isolate executor drift: before=%+v after=%+v", before, after)
	}
}

func TestCLIStandardCapturesProcessStreams(t *testing.T) {
	originalStdin, originalStderr := os.Stdin, os.Stderr
	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	promptReader, promptWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	replacementIn, replacementInWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	replacementErrReader, replacementErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Stdin, os.Stderr = originalStdin, originalStderr
		for _, file := range []*os.File{inputReader, inputWriter, promptReader, promptWriter, replacementIn, replacementInWriter, replacementErrReader, replacementErr} {
			_ = file.Close()
		}
	})
	os.Stdin, os.Stderr = inputReader, promptWriter
	definitions, err := Standard(Options{UserSurface: userinteract.SurfaceCLI})
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin, os.Stderr = replacementIn, replacementErr
	if _, writeErr := inputWriter.WriteString("captured answer\n\n"); writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr := inputWriter.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	output := invokeAt(t, findDefinition(t, definitions, IDUserInteract), "", `{"question":"Question?"}`)
	if !strings.Contains(output, "captured answer") {
		t.Fatalf("user interaction output = %s", output)
	}
	if closeErr := promptWriter.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	prompt, err := io.ReadAll(promptReader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), "Question?") {
		t.Fatalf("captured stderr prompt = %q", prompt)
	}
}

func TestEnvironmentNormalization(t *testing.T) {
	got, err := normalizeEnvironment([]string{"Z=first", "A=value", "Z=last"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A=value", "Z=last"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeEnvironment = %#v, want %#v", got, want)
	}
	for _, invalid := range [][]string{{"MISSING_SEPARATOR"}, {"=empty"}, {"NUL=x\x00y"}} {
		if _, err := normalizeEnvironment(invalid); err == nil {
			t.Errorf("normalizeEnvironment(%q) accepted invalid entry", invalid)
		}
	}
}

func TestURLFetchOptionContainerIsCaptured(t *testing.T) {
	firstTransport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("first")), Header: make(http.Header)}, nil
	})
	secondTransport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("second")), Header: make(http.Header)}, nil
	})
	options := &urlfetch.Options{HTTPClient: &http.Client{Transport: firstTransport}}
	definitions, err := Standard(Options{URLFetchOptions: options})
	if err != nil {
		t.Fatal(err)
	}
	options.HTTPClient = &http.Client{Transport: secondTransport}
	output := invokeAt(t, findDefinition(t, definitions, IDURLFetch), "", `{"url":"https://example.test/value"}`)
	if !strings.Contains(output, "first") || strings.Contains(output, "second") {
		t.Fatalf("url fetch did not retain captured client: %s", output)
	}
}

func TestOptionValidationIsAtomic(t *testing.T) {
	var typedNilTracker *closeWriter
	var writer tracker.CloseWriter = typedNilTracker
	var typedNilReader *nilReader
	var typedNilWriter *nilWriter
	tests := []Options{
		{UserSurface: userinteract.Surface("invalid")},
		{ShellOptions: &shell.Options{OutputCapBytes: -1}},
		{TrackerWriter: writer},
		{UserSurface: userinteract.SurfaceCLI, UserOptions: userinteract.Options{Stdin: typedNilReader}},
		{UserSurface: userinteract.SurfaceCLI, UserOptions: userinteract.Options{Stderr: typedNilWriter}},
		{SearchOptions: &search.Options{Env: []string{"INVALID"}}},
	}
	for index, options := range tests {
		definitions, err := Standard(options)
		if err == nil || definitions != nil {
			t.Errorf("case %d = %#v, %v; want nil error", index, definitionIDs(definitions), err)
		}
	}
}

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

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func definitionIDs(definitions []Definition) []string {
	ids := make([]string, len(definitions))
	for index := range definitions {
		ids[index] = definitions[index].ID
	}
	return ids
}

func findDefinition(t *testing.T, definitions []Definition, id string) Definition {
	t.Helper()
	for _, definition := range definitions {
		if definition.ID == id {
			return definition
		}
	}
	t.Fatalf("definition %s not found", id)
	return Definition{}
}

func invokeAt(t *testing.T, definition Definition, root, arguments string) string {
	t.Helper()
	instance := Instance{WorkspaceRoot: root}
	toolInstance, err := definition.New(context.Background(), instance)
	if err != nil {
		t.Fatalf("New %s: %v", definition.ID, err)
	}
	output, err := toolInstance.InvokableRun(context.Background(), arguments)
	if err != nil {
		t.Fatalf("Invoke %s: %v", definition.ID, err)
	}
	return output
}

func invokePairContains(t *testing.T, definitions []Definition, id, rootA, rootB, arguments, wantA, wantB string) {
	t.Helper()
	definition := findDefinition(t, definitions, id)
	outputA := invokeAt(t, definition, rootA, arguments)
	outputB := invokeAt(t, definition, rootB, arguments)
	if !strings.Contains(outputA, wantA) || strings.Contains(outputA, wantB) {
		t.Errorf("%s root A output = %s", id, outputA)
	}
	if !strings.Contains(outputB, wantB) || strings.Contains(outputB, wantA) {
		t.Errorf("%s root B output = %s", id, outputB)
	}
}

func writeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, relative), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, root, relative, expected string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != expected {
		t.Fatalf("%s content = %q, want %q", relative, content, expected)
	}
}
