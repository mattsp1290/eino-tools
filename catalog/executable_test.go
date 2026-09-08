//go:build unix

package catalog

import (
	"context"
	"encoding/json"
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
	shellOptions := &shell.Options{StartupMode: shell.StartupModeNonLogin, ShellBinary: "/bin/sh", Env: []string{"PATH=/usr/bin:/bin", "CATALOG_VALUE=one"}}
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
		ShellOptions:  &shell.Options{StartupMode: shell.StartupModeNonLogin, ShellBinary: "/bin/sh", Env: []string{"PATH=/usr/bin:/bin"}},
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
		ShellOptions:  &shell.Options{StartupMode: shell.StartupModeNonLogin, ShellBinary: "/bin/sh", Env: []string{"PATH=/usr/bin:/bin"}},
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
		ShellOptions:  &shell.Options{StartupMode: shell.StartupModeNonLogin, ShellBinary: invocationPath, Env: []string{"PATH=/usr/bin:/bin"}},
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

func TestShellPolicyCaptureAndIdentity(t *testing.T) {
	rgPath, lookupErr := exec.LookPath("rg")
	if lookupErr != nil {
		t.Fatal(lookupErr)
	}
	base := shell.Options{ShellBinary: "/bin/sh", Env: []string{"HOME=" + canonicalTempDir(t), "PATH=/usr/bin:/bin", "VALUE=original"}}
	build := func(options *shell.Options) Definition {
		definitions, err := Standard(Options{SearchOptions: &search.Options{RGBinary: rgPath, Env: []string{"PATH=/usr/bin:/bin"}}, ShellOptions: options})
		if err != nil {
			t.Fatal(err)
		}
		return findDefinition(t, definitions, IDShell)
	}
	defaultDef := build(&base)
	base.StartupMode = shell.StartupModeLogin
	loginDef := build(&base)
	if defaultDef.ExecutorHash != loginDef.ExecutorHash {
		t.Fatal("default/login hash mismatch")
	}
	base.StartupMode = shell.StartupModeNonLogin
	nonLoginDef := build(&base)
	if nonLoginDef.ExecutorHash == loginDef.ExecutorHash || nonLoginDef.SchemaHash != loginDef.SchemaHash {
		t.Fatal("mode identity mismatch")
	}
	base.OutputCapBytes = shell.DefaultOutputCapBytes
	if build(&base).ExecutorHash != nonLoginDef.ExecutorHash {
		t.Fatal("effective default cap mismatch")
	}
	base.OutputCapBytes = 8
	captured := build(&base)
	if captured.ExecutorHash == nonLoginDef.ExecutorHash {
		t.Fatal("cap missing from identity")
	}
	base.Env[0], base.Env[2] = "HOME=/missing", "VALUE=mutated"
	base.StartupMode, base.ShellBinary, base.OutputCapBytes = shell.StartupModeLogin, "/missing", 100
	for i := 0; i < 2; i++ {
		raw := invokeAt(t, captured, canonicalTempDir(t), `{"cmd":"printf '%s' \"$VALUE-012345\"; printf abcdefghijkl >&2","startup_mode":"login","env":["VALUE=model"],"shell_binary":"/missing"}`)
		var got shell.Result
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatal(err)
		}
		if got.Stdout != "original" || got.Stderr != "abcdefgh" || !got.StdoutTruncated || !got.StderrTruncated || got.Error != nil || got.ExitCode != 0 {
			t.Fatalf("captured policy changed: %+v", got)
		}
		base = shell.Options{Env: []string{"VALUE=again"}}
	}
	reordered := shell.Options{ShellBinary: "/bin/sh", StartupMode: shell.StartupModeNonLogin, Env: []string{"B=two", "A=one"}}
	first := build(&reordered)
	reordered.Env = []string{"A=one", "B=two"}
	if first.ExecutorHash != build(&reordered).ExecutorHash {
		t.Fatal("environment order changed hash")
	}
	marker := filepath.Join(canonicalTempDir(t), "launched")
	recorder := filepath.Join(canonicalTempDir(t), "recorder")
	if err := os.WriteFile(recorder, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	definitions, err := Standard(Options{SearchOptions: &search.Options{RGBinary: rgPath}, ShellOptions: &shell.Options{ShellBinary: recorder, StartupMode: "invalid"}})
	if err == nil || definitions != nil || !strings.Contains(err.Error(), "startup mode") {
		t.Fatalf("invalid policy = %v %v", definitions, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("validation launched shell")
	}
}

func TestCapturedShellEmptyAndNilEnvironment(t *testing.T) {
	t.Setenv("CATALOG_PARENT_CANARY", "before")
	t.Setenv("HOME", canonicalTempDir(t))
	t.Setenv("BASH_ENV", "")
	t.Setenv("ENV", "")
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		t.Fatal(err)
	}
	for _, empty := range []bool{true, false} {
		t.Setenv("CATALOG_PARENT_CANARY", "before")
		options := &shell.Options{ShellBinary: "/bin/sh", StartupMode: shell.StartupModeNonLogin}
		if empty {
			options.Env = []string{}
		}
		definitions, err := Standard(Options{SearchOptions: &search.Options{RGBinary: rgPath}, ShellOptions: options})
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("CATALOG_PARENT_CANARY", "after")
		for i := 0; i < 2; i++ {
			raw := invokeAt(t, findDefinition(t, definitions, IDShell), canonicalTempDir(t), `{"cmd":"printf '%s' \"${CATALOG_PARENT_CANARY-unset}\""}`)
			var got shell.Result
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatal(err)
			}
			want := "before"
			if empty {
				want = "unset"
			}
			if got.Stdout != want || got.Error != nil || got.ExitCode != 0 {
				t.Fatalf("empty=%t: %+v", empty, got)
			}
		}
	}
}
