//go:build unix

package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
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
