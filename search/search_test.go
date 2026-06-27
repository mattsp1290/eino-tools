package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
)

func newTestTool(t *testing.T) (*Tool, string) {
	t.Helper()

	root := t.TempDir()
	tool, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tool, root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestNewRejectsInvalidWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := New(""); err == nil {
		t.Fatal("New(empty) returned nil error")
	}
	if _, err := New("relative"); err == nil {
		t.Fatal("New(relative) returned nil error")
	}
	if _, err := New(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("New(missing) returned nil error")
	}
}

func TestRunPathDefaultsToWorkspaceRoot(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "src/main.go", "package main\nfunc main() { println(\"needle\") }\n")

	res := tool.Run(context.Background(), Args{Pattern: "needle"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 1 {
		t.Fatalf("MatchCount = %d, want 1: %+v", res.MatchCount, res.Matches)
	}
	if res.Matches[0].Path != "src/main.go" {
		t.Fatalf("match path = %q, want src/main.go", res.Matches[0].Path)
	}
}

func TestRunPathScopesSearch(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "a/one.txt", "needle in a\n")
	writeFile(t, root, "b/two.txt", "needle in b\n")

	res := tool.Run(context.Background(), Args{Pattern: "needle", Path: "a"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 1 || res.Matches[0].Path != "a/one.txt" {
		t.Fatalf("matches = %+v, want one match in a/one.txt", res.Matches)
	}
}

func TestRunSearchFileAsRoot(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "one.txt", "needle\n")
	writeFile(t, root, "two.txt", "needle\n")

	res := tool.Run(context.Background(), Args{Pattern: "needle", Path: "one.txt"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 1 || res.Matches[0].Path != "one.txt" {
		t.Fatalf("matches = %+v, want one match in one.txt", res.Matches)
	}
}

func TestRunNoMatchesIsSuccess(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "file.txt", "haystack\n")

	res := tool.Run(context.Background(), Args{Pattern: "needle"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 0 || len(res.Matches) != 0 {
		t.Fatalf("matches = %+v count=%d, want empty", res.Matches, res.MatchCount)
	}
}

func TestRunRejectsTraversal(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	outside := filepath.Dir(root)
	if err := os.WriteFile(filepath.Join(outside, "outside-search-probe.txt"), []byte("needle\n"), 0o600); err != nil {
		t.Fatalf("write outside probe: %v", err)
	}

	res := tool.Run(context.Background(), Args{Pattern: "needle", Path: ".."})
	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryPathEscape {
		t.Fatalf("Error = %+v, want path_escape", res.Error)
	}
	if res.MatchCount != 0 {
		t.Fatalf("MatchCount = %d, want 0", res.MatchCount)
	}
}

func TestRunRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	tool, root := newTestTool(t)
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("needle\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	link := filepath.Join(root, "escape.txt")
	if err := os.Symlink(outsideFile, link); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink not permitted: %v", err)
		}
		t.Fatalf("Symlink: %v", err)
	}

	res := tool.Run(context.Background(), Args{Pattern: "needle", Path: "escape.txt"})
	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryPathEscape {
		t.Fatalf("Error = %+v, want path_escape", res.Error)
	}
	if res.MatchCount != 0 {
		t.Fatalf("MatchCount = %d, want 0", res.MatchCount)
	}
}

func TestRunRejectsAbsoluteAndNULPath(t *testing.T) {
	t.Parallel()

	tool, _ := newTestTool(t)
	for _, path := range []string{filepath.Join(string(filepath.Separator), "tmp"), "a\x00b"} {
		res := tool.Run(context.Background(), Args{Pattern: "needle", Path: path})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("path=%q Outcome = %q, want failed", path, res.Outcome)
		}
		if res.Error == nil || res.Error.Category != ErrCategoryValidation {
			t.Fatalf("path=%q Error = %+v, want validation", path, res.Error)
		}
	}
}

func TestRunMissingPath(t *testing.T) {
	t.Parallel()

	tool, _ := newTestTool(t)
	res := tool.Run(context.Background(), Args{Pattern: "needle", Path: "missing"})
	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryNotFound {
		t.Fatalf("Error = %+v, want not_found", res.Error)
	}
	if !strings.Contains(res.Error.Message, "missing") {
		t.Fatalf("Error.Message = %q, want missing path context", res.Error.Message)
	}
}

func TestRunPathCleaningNormalizes(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "src/file.txt", "needle\n")

	res := tool.Run(context.Background(), Args{Pattern: "needle", Path: "src/./nested/../"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 1 || res.Matches[0].Path != "src/file.txt" {
		t.Fatalf("matches = %+v, want src/file.txt", res.Matches)
	}
}

func TestRunRichControls(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "src/one.go", "before\nNeedle\nAfter\n")
	writeFile(t, root, "src/two.txt", "before\nNeedle\nAfter\n")

	res := tool.Run(context.Background(), Args{
		Pattern:    "needle",
		Glob:       Globs{"**/*.go"},
		Literal:    true,
		IgnoreCase: true,
		Context:    1,
		Limit:      1,
	})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 1 || res.Matches[0].Path != "src/one.go" {
		t.Fatalf("matches = %+v, want one src/one.go match", res.Matches)
	}
	if len(res.Matches[0].Submatches) != 0 {
		t.Fatalf("literal search emitted submatches: %+v", res.Matches[0].Submatches)
	}
	if len(res.Matches[0].Before) != 1 || res.Matches[0].Before[0].Line != "before" {
		t.Fatalf("before context = %+v", res.Matches[0].Before)
	}
	if len(res.Matches[0].After) != 1 || res.Matches[0].After[0].Line != "After" {
		t.Fatalf("after context = %+v", res.Matches[0].After)
	}
}

func TestRunContextLinesAttachToNearestMatchSide(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "ctx.txt", "match one\nafter one\ngap\nbefore two\nmatch two\n")

	res := tool.Run(context.Background(), Args{Pattern: "match", Path: "ctx.txt", Context: 1})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.MatchCount != 2 {
		t.Fatalf("MatchCount = %d, want 2: %+v", res.MatchCount, res.Matches)
	}
	if len(res.Matches[0].After) != 1 || res.Matches[0].After[0].Line != "after one" {
		t.Fatalf("first after context = %+v", res.Matches[0].After)
	}
	if len(res.Matches[0].Before) != 0 {
		t.Fatalf("first before context = %+v", res.Matches[0].Before)
	}
	if len(res.Matches[1].Before) != 1 || res.Matches[1].Before[0].Line != "before two" {
		t.Fatalf("second before context = %+v", res.Matches[1].Before)
	}
	if len(res.Matches[1].After) != 0 {
		t.Fatalf("second after context = %+v", res.Matches[1].After)
	}
}

func TestRunLimitPreservesTrailingContextForReturnedMatch(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "limited-context.txt", "match one\nafter one\nmatch two\n")

	res := tool.Run(context.Background(), Args{Pattern: "match", Path: "limited-context.txt", Context: 1, Limit: 1})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if !res.Truncated || res.TruncationReason != "matches" || res.MatchCount != 1 {
		t.Fatalf("truncation = %t/%q count=%d", res.Truncated, res.TruncationReason, res.MatchCount)
	}
	if len(res.Matches[0].After) != 1 || res.Matches[0].After[0].Line != "after one" {
		t.Fatalf("after context = %+v", res.Matches[0].After)
	}
}

func TestRunLimitTruncates(t *testing.T) {
	t.Parallel()

	tool, root := newTestTool(t)
	writeFile(t, root, "a.txt", "needle\n")
	writeFile(t, root, "b.txt", "needle\n")

	res := tool.Run(context.Background(), Args{Pattern: "needle", Limit: 1})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if !res.Truncated || res.TruncationReason != "matches" || res.MatchCount != 1 {
		t.Fatalf("truncation = %t/%q count=%d", res.Truncated, res.TruncationReason, res.MatchCount)
	}
}

func TestGlobArgumentAcceptsStringOrArray(t *testing.T) {
	t.Parallel()

	var one struct {
		Glob Globs `json:"glob"`
	}
	if err := json.Unmarshal([]byte(`{"glob":"*.go"}`), &one); err != nil {
		t.Fatalf("unmarshal string glob: %v", err)
	}
	if !slices.Equal(one.Glob, Globs{"*.go"}) {
		t.Fatalf("string glob = %#v", one.Glob)
	}

	var many struct {
		Glob Globs `json:"glob"`
	}
	if err := json.Unmarshal([]byte(`{"glob":["*.go","*.md"]}`), &many); err != nil {
		t.Fatalf("unmarshal array glob: %v", err)
	}
	if !slices.Equal(many.Glob, Globs{"*.go", "*.md"}) {
		t.Fatalf("array glob = %#v", many.Glob)
	}
}

func TestSchemaABI(t *testing.T) {
	t.Parallel()

	var got struct {
		Type                 string                     `json:"type"`
		AdditionalProperties bool                       `json:"additionalProperties"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
	}
	if err := json.Unmarshal(Schema(), &got); err != nil {
		t.Fatalf("schema parse: %v", err)
	}
	if got.Type != "object" {
		t.Fatalf("type = %q, want object", got.Type)
	}
	if got.AdditionalProperties {
		t.Fatal("additionalProperties = true, want false")
	}
	for _, property := range []string{"pattern", "path", "timeout_seconds", "glob", "literal", "ignore_case", "context", "limit"} {
		if _, ok := got.Properties[property]; !ok {
			t.Fatalf("schema missing %q property", property)
		}
	}
	if len(got.Required) != 1 || got.Required[0] != "pattern" {
		t.Fatalf("required = %#v, want [pattern]", got.Required)
	}

	var pattern struct {
		Type      string `json:"type"`
		MinLength int    `json:"minLength"`
	}
	if err := json.Unmarshal(got.Properties["pattern"], &pattern); err != nil {
		t.Fatalf("pattern schema parse: %v", err)
	}
	if pattern.Type != "string" || pattern.MinLength != 1 {
		t.Fatalf("pattern schema = %+v, want string minLength 1", pattern)
	}

	var timeout struct {
		Type    string `json:"type"`
		Minimum int    `json:"minimum"`
		Maximum int    `json:"maximum"`
	}
	if err := json.Unmarshal(got.Properties["timeout_seconds"], &timeout); err != nil {
		t.Fatalf("timeout_seconds schema parse: %v", err)
	}
	if timeout.Type != "integer" || timeout.Minimum != 0 || timeout.Maximum != MaxTimeoutSeconds {
		t.Fatalf("timeout_seconds schema = %+v, want integer min 0 max %d", timeout, MaxTimeoutSeconds)
	}
}

func TestSchemaReturnsFreshSlice(t *testing.T) {
	t.Parallel()

	a := Schema()
	b := Schema()
	if len(a) > 0 && len(b) > 0 && &a[0] == &b[0] {
		t.Fatal("Schema returned aliased slices")
	}
	a[0] = 'X'
	if Schema()[0] == 'X' {
		t.Fatal("mutating Schema return affected later call")
	}
}

func TestNameAndConstantsMatchSourceABI(t *testing.T) {
	t.Parallel()

	if Name != "search" {
		t.Fatalf("Name = %q, want search", Name)
	}

	want := []int{60, 600, 200, 4 * 1024, 256 * 1024, 4 * 1024}
	got := []int{
		DefaultTimeoutSeconds,
		MaxTimeoutSeconds,
		MaxMatches,
		MaxLineBytes,
		MaxResultBytes,
		MaxStderrBytes,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("constants = %#v, want %#v", got, want)
	}
}

func TestInvokableRunRejectsDuplicateTopLevelKeysBeforeExecution(t *testing.T) {
	t.Parallel()

	var tool *Tool
	for _, in := range []string{
		`{"pattern":"a","pattern":"b"}`,
		`{"pattern":"a","path":".","path":"src"}`,
		`{"pattern":"a","timeout_seconds":1,"timeout_seconds":2}`,
	} {
		_, err := tool.InvokableRun(context.Background(), in)
		if err == nil {
			t.Fatalf("InvokableRun(%s) returned nil error", in)
		}
		if !strings.Contains(err.Error(), "duplicate top-level key") {
			t.Fatalf("InvokableRun(%s) err = %q, want duplicate-key error", in, err.Error())
		}
	}
}

func TestInvokableRunRejectsParseErrorsBeforeExecution(t *testing.T) {
	t.Parallel()

	var tool *Tool
	for _, in := range []string{"", "   ", "{not-json"} {
		_, err := tool.InvokableRun(context.Background(), in)
		if err == nil {
			t.Fatalf("InvokableRun(%q) returned nil error", in)
		}
	}
}

func TestResultPreservesUnknownFieldsInRawJSON(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"outcome":"succeeded","matches":[],"match_count":0,"duration_ms":1,"future":"field"}`)
	var got Result
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Contains(got.RawJSON, []byte(`"future"`)) {
		t.Fatalf("RawJSON = %s, want future field preserved", got.RawJSON)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(encoded, []byte("RawJSON")) || bytes.Contains(encoded, []byte("future")) {
		t.Fatalf("marshal emitted raw/future field: %s", encoded)
	}
}
