package glob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
)

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestRunMatchesSortedHiddenAndSkipsVCS(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, "a.go", "package a\n")
	writeFixture(t, root, "pkg/b_test.go", "package pkg\n")
	writeFixture(t, root, ".hidden/c.go", "package hidden\n")
	writeFixture(t, root, ".git/config.go", "ignored\n")
	if err := os.MkdirAll(filepath.Join(root, "pkg", "dir.go"), 0o700); err != nil {
		t.Fatalf("mkdir dir fixture: %v", err)
	}

	tool, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Pattern: "**/*.go"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q err=%+v", res.Outcome, res.Error)
	}
	got := pathsOnly(res.Paths)
	want := []string{".hidden/c.go", "a.go", "pkg/b_test.go", "pkg/dir.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
	if !res.Paths[3].IsDir {
		t.Fatalf("directory match missing is_dir: %+v", res.Paths[3])
	}
}

func TestRunPathScopeAndLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, "pkg/a.go", "a\n")
	writeFixture(t, root, "pkg/b.go", "b\n")
	writeFixture(t, root, "other/c.go", "c\n")
	tool, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res := tool.Run(context.Background(), Args{Pattern: "*.go", Path: "pkg", Limit: 1})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q err=%+v", res.Outcome, res.Error)
	}
	if res.Count != 1 || !res.Truncated || len(res.Paths) != 1 || !strings.HasPrefix(res.Paths[0].Path, "pkg/") {
		t.Fatalf("limited scoped result = %+v", res)
	}
}

func TestRunRejectsPathAndSymlinkEscapes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, outside, "secret.txt", "secret\n")
	tool, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Pattern: "*", Path: ".."})
	assertResultCategory(t, res, ErrCategoryPathEscape)

	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatalf("Symlink: %v", err)
	}
	res = tool.Run(context.Background(), Args{Pattern: "*"})
	assertResultCategory(t, res, ErrCategoryPathEscape)
}

func TestSchemaInvokableAndRawJSON(t *testing.T) {
	t.Parallel()

	a := Schema()
	b := Schema()
	a[0] = ' '
	if b[0] == ' ' {
		t.Fatal("Schema returned aliased slices")
	}
	var schema struct {
		AdditionalProperties bool                       `json:"additionalProperties"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatalf("schema parse: %v", err)
	}
	if schema.AdditionalProperties {
		t.Fatal("additionalProperties = true")
	}
	for _, property := range []string{"pattern", "path", "limit"} {
		if _, ok := schema.Properties[property]; !ok {
			t.Fatalf("missing schema property %q", property)
		}
	}

	var nilTool *Tool
	if _, err := nilTool.InvokableRun(context.Background(), `{"pattern":"*","pattern":"**"}`); err == nil ||
		!strings.Contains(err.Error(), "duplicate top-level key") {
		t.Fatalf("duplicate key err = %v", err)
	}

	raw := []byte(`{"outcome":"succeeded","paths":[],"count":0,"future":"field"}`)
	var got Result
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Contains(got.RawJSON, []byte("future")) {
		t.Fatalf("RawJSON = %s", got.RawJSON)
	}
}

func pathsOnly(entries []PathEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Path)
	}
	return out
}

func assertResultCategory(t *testing.T, res Result, want string) {
	t.Helper()
	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != want {
		t.Fatalf("Error = %+v, want %s", res.Error, want)
	}
}
