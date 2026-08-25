//go:build unix

package catalog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
)

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

func TestCatalogPreservesExecutableInvocationPath(t *testing.T) {
	rgPath, lookupErr := exec.LookPath("rg")
	if lookupErr != nil {
		t.Skip("rg unavailable")
	}
	temp := canonicalTempDir(t)
	targetPath := filepath.Join(temp, "target-shell")
	invocationPath := filepath.Join(temp, "shell-alias")
	if err := os.WriteFile(targetPath, []byte("#!/bin/sh\nprintf '%s' \"$0\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, invocationPath); err != nil {
		t.Fatal(err)
	}
	definitions, err := Standard(Options{
		SearchOptions: &search.Options{RGBinary: rgPath, Env: []string{"PATH=/usr/bin:/bin"}},
		ShellOptions:  &shell.Options{ShellBinary: invocationPath, Env: []string{"PATH=/usr/bin:/bin"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := invokeAt(t, findDefinition(t, definitions, IDShell), canonicalTempDir(t), `{"cmd":"ignored"}`)
	if !strings.Contains(output, invocationPath) || strings.Contains(output, targetPath) {
		t.Fatalf("shell invocation path was not preserved: %s", output)
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
	for _, invalid := range [][]string{
		{"MISSING_SEPARATOR"},
		{"=empty"},
		{"NUL=x\x00y"},
		{string([]byte{'N', 'O', 'N', 'U', 'T', 'F', '8', '=', 0xff})},
	} {
		if _, err := normalizeEnvironment(invalid); err == nil {
			t.Errorf("normalizeEnvironment(%q) accepted invalid entry", invalid)
		}
	}
	invalidPath := string(append([]byte("/tmp/invalid-"), 0xff))
	if _, _, _, err := resolveExecutable(invalidPath, []string{"PATH=/usr/bin:/bin"}); err == nil {
		t.Fatal("resolveExecutable accepted a non-UTF-8 path")
	}
}
