package fileops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
)

func TestBaseResultIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   BaseResult
		want bool
	}{
		{name: "success", in: BaseResult{Outcome: result.OutcomeSucceeded}, want: false},
		{name: "failed without error", in: BaseResult{Outcome: result.OutcomeFailed}, want: false},
		{name: "io", in: BaseResult{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryIO}}, want: true},
		{name: "unknown", in: BaseResult{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryUnknown}}, want: true},
		{name: "validation", in: BaseResult{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryValidation}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.in.IsRetryable(); got != tt.want {
				t.Fatalf("IsRetryable() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPathValidationAndContainment(t *testing.T) {
	t.Parallel()

	if err := validateWorkspacePath(""); err == nil {
		t.Fatal("validateWorkspacePath(empty) returned nil")
	}
	if err := validateWorkspacePath("relative"); err == nil {
		t.Fatal("validateWorkspacePath(relative) returned nil")
	}
	if err := validateWorkspacePath(t.TempDir()); err != nil {
		t.Fatalf("validateWorkspacePath(abs): %v", err)
	}

	for _, rel := range []string{"", filepath.Join(string(filepath.Separator), "tmp"), "a\x00b"} {
		if got := validateRelPath(rel); got == nil {
			t.Fatalf("validateRelPath(%q) returned nil", rel)
		}
	}
	if got := validateRelPath("a/b.txt"); got != nil {
		t.Fatalf("validateRelPath(valid): %v", got)
	}

	root := filepath.Clean(filepath.Join(t.TempDir(), "root"))
	child := filepath.Join(root, "a", "b")
	if !syntacticDescendant(root, child) {
		t.Fatal("syntacticDescendant(root, child) = false")
	}
	if syntacticDescendant(root, filepath.Join(root, "..", "outside")) {
		t.Fatal("syntacticDescendant allowed parent escape")
	}
	if isDescendant(root, root, false) {
		t.Fatal("isDescendant allowed root when allowRoot=false")
	}
	if !isDescendant(root, root, true) {
		t.Fatal("isDescendant rejected root when allowRoot=true")
	}
}

func TestResolveExistingAndWritable(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	workspace = canonicalizeWorkspace(workspace)
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	resolved, perr := resolveExisting(workspace, "a.txt", false)
	if perr != nil {
		t.Fatalf("resolveExisting existing: %v", perr)
	}
	if !strings.HasSuffix(resolved, "a.txt") {
		t.Fatalf("resolved = %q, want a.txt suffix", resolved)
	}

	if _, missingErr := resolveExisting(workspace, "missing.txt", false); missingErr == nil || missingErr.category != ErrCategoryNotFound {
		t.Fatalf("resolveExisting missing = %#v, want not_found", missingErr)
	}
	if _, escapeErr := resolveExisting(workspace, "..", true); escapeErr == nil || escapeErr.category != ErrCategoryPathEscape {
		t.Fatalf("resolveExisting escape = %#v, want path_escape", escapeErr)
	}

	writable, perr := resolveWritable(workspace, "nested/out.txt", true)
	if perr != nil {
		t.Fatalf("resolveWritable create dirs: %v", perr)
	}
	if !strings.HasSuffix(writable, filepath.Join("nested", "out.txt")) {
		t.Fatalf("writable = %q, want nested/out.txt suffix", writable)
	}
	if _, err := os.Stat(filepath.Join(workspace, "nested")); err != nil {
		t.Fatalf("expected parent directory to be created: %v", err)
	}

	if _, perr := resolveWritable(workspace, "other/out.txt", false); perr == nil || perr.category != ErrCategoryNotFound {
		t.Fatalf("resolveWritable missing parent = %#v, want not_found", perr)
	}
}

func TestRejectDuplicateTopLevelKeys(t *testing.T) {
	t.Parallel()

	if err := rejectDuplicateTopLevelKeys([]byte(`{"path":"a","path":"b"}`)); err == nil {
		t.Fatal("duplicate top-level key returned nil")
	}
	if err := rejectDuplicateTopLevelKeys([]byte(`{"path":"a","nested":{"path":"b"}}`)); err != nil {
		t.Fatalf("nested duplicate-like key: %v", err)
	}
}

func TestFailureHelpersAndContextCategory(t *testing.T) {
	t.Parallel()

	base := failed(ErrCategoryValidation, "bad input")
	if base.Outcome != result.OutcomeFailed {
		t.Fatalf("failed outcome = %q, want failed", base.Outcome)
	}
	if base.Error == nil || base.Error.Category != ErrCategoryValidation || base.Error.Message != "bad input" {
		t.Fatalf("failed error = %#v", base.Error)
	}

	perr := newPathErr(ErrCategoryPathEscape, "escape %s", "x")
	if perr.Error() != "escape x" {
		t.Fatalf("pathErr Error() = %q", perr.Error())
	}
	fromPath := failedFromPathErr(perr)
	if fromPath.Error == nil || fromPath.Error.Category != ErrCategoryPathEscape {
		t.Fatalf("failedFromPathErr = %#v", fromPath.Error)
	}

	if got := contextErrCategory(context.Canceled); got != ErrCategoryCanceled {
		t.Fatalf("contextErrCategory(canceled) = %q", got)
	}
	if got := contextErrCategory(errors.New("boom")); got != ErrCategoryUnknown {
		t.Fatalf("contextErrCategory(other) = %q", got)
	}
}

func TestBuildToolInfo(t *testing.T) {
	t.Parallel()

	info, err := buildToolInfo("file_test", "test tool", []byte(`{"type":"object","additionalProperties":false}`))
	if err != nil {
		t.Fatalf("buildToolInfo valid: %v", err)
	}
	if info.Name != "file_test" || info.Desc != "test tool" || info.ParamsOneOf == nil {
		t.Fatalf("info = %#v", info)
	}

	if _, err := buildToolInfo("bad", "bad", []byte(`{`)); err == nil {
		t.Fatal("buildToolInfo invalid schema returned nil")
	}
}

func TestToolNamesAndSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema func() json.RawMessage
		want   string
	}{
		{name: "read", schema: ReadSchema, want: NameRead},
		{name: "write", schema: WriteSchema, want: NameWrite},
		{name: "edit", schema: EditSchema, want: NameEdit},
		{name: "list", schema: ListSchema, want: NameList},
	}

	wantNames := map[string]string{
		"read":  "file_read",
		"write": "file_write",
		"edit":  "file_edit",
		"list":  "file_list",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.want != wantNames[tt.name] {
				t.Fatalf("name = %q, want %q", tt.want, wantNames[tt.name])
			}

			first := tt.schema()
			second := tt.schema()
			if len(first) == 0 || len(second) == 0 {
				t.Fatal("schema returned empty JSON")
			}
			first[0] = ' '
			if second[0] == ' ' {
				t.Fatal("schema returned aliased slice")
			}

			var probe struct {
				AdditionalProperties bool `json:"additionalProperties"`
			}
			if err := json.Unmarshal(second, &probe); err != nil {
				t.Fatalf("schema JSON invalid: %v", err)
			}
			if probe.AdditionalProperties {
				t.Fatal("additionalProperties = true, want false")
			}
		})
	}
}

func TestInvokableRunRejectsBadJSON(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	read, err := NewReadTool(workspace)
	if err != nil {
		t.Fatalf("NewReadTool: %v", err)
	}
	write, err := NewWriteTool(workspace)
	if err != nil {
		t.Fatalf("NewWriteTool: %v", err)
	}
	edit, err := NewEditTool(workspace)
	if err != nil {
		t.Fatalf("NewEditTool: %v", err)
	}
	list, err := NewListTool(workspace)
	if err != nil {
		t.Fatalf("NewListTool: %v", err)
	}

	tests := []struct {
		name string
		run  func(context.Context, string) (string, error)
	}{
		{name: "read", run: func(ctx context.Context, args string) (string, error) {
			return read.InvokableRun(ctx, args)
		}},
		{name: "write", run: func(ctx context.Context, args string) (string, error) {
			return write.InvokableRun(ctx, args)
		}},
		{name: "edit", run: func(ctx context.Context, args string) (string, error) {
			return edit.InvokableRun(ctx, args)
		}},
		{name: "list", run: func(ctx context.Context, args string) (string, error) {
			return list.InvokableRun(ctx, args)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := tt.run(context.Background(), `{not json`); err == nil {
				t.Fatal("malformed JSON returned nil error")
			}
			if _, err := tt.run(context.Background(), `{"path":"a","path":"b"}`); err == nil ||
				!strings.Contains(err.Error(), "duplicate top-level key") {
				t.Fatalf("duplicate key err = %v, want duplicate top-level key", err)
			}
		})
	}
}
