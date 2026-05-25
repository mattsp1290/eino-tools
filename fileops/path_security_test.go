package fileops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
)

func TestFileopsRejectPathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatalf("create outside dir: %v", err)
	}
	outsideFile := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}

	read := mustNewReadTool(t, workspace)
	write := mustNewWriteTool(t, workspace)
	edit := mustNewEditTool(t, workspace)
	list := mustNewListTool(t, workspace)

	assertCategory(t, read.Run(context.Background(), ReadArgs{Path: "../outside.txt"}).BaseResult, ErrCategoryPathEscape)
	assertCategory(t, write.Run(context.Background(), WriteArgs{Path: "../outside.txt", Content: "changed"}).BaseResult, ErrCategoryPathEscape)
	assertCategory(t, edit.Run(context.Background(), EditArgs{Path: "../outside.txt", Anchor: "outside", Replacement: "changed"}).BaseResult, ErrCategoryPathEscape)
	assertCategory(t, list.Run(context.Background(), ListArgs{Path: "../outside"}).BaseResult, ErrCategoryPathEscape)

	got, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("read outside fixture: %v", err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside file changed to %q", got)
	}
}

func TestFileopsRejectAbsolutePaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	absolute := filepath.Join(workspace, "inside.txt")
	if err := os.WriteFile(absolute, []byte("inside"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	read := mustNewReadTool(t, workspace)
	write := mustNewWriteTool(t, workspace)
	edit := mustNewEditTool(t, workspace)
	list := mustNewListTool(t, workspace)

	assertCategory(t, read.Run(context.Background(), ReadArgs{Path: absolute}).BaseResult, ErrCategoryValidation)
	assertCategory(t, write.Run(context.Background(), WriteArgs{Path: absolute, Content: "changed"}).BaseResult, ErrCategoryValidation)
	assertCategory(t, edit.Run(context.Background(), EditArgs{Path: absolute, Anchor: "inside", Replacement: "changed"}).BaseResult, ErrCategoryValidation)
	assertCategory(t, list.Run(context.Background(), ListArgs{Path: absolute}).BaseResult, ErrCategoryValidation)

	got, err := os.ReadFile(absolute)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(got) != "inside" {
		t.Fatalf("inside file changed to %q", got)
	}
}

func TestFileopsRejectSymlinkEscapes(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(workspace, "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "outdir")); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}

	read := mustNewReadTool(t, workspace)
	write := mustNewWriteTool(t, workspace)
	edit := mustNewEditTool(t, workspace)
	list := mustNewListTool(t, workspace)

	assertCategory(t, read.Run(context.Background(), ReadArgs{Path: "link.txt"}).BaseResult, ErrCategoryPathEscape)
	assertCategory(t, write.Run(context.Background(), WriteArgs{Path: "link.txt", Content: "changed"}).BaseResult, ErrCategoryPathEscape)
	assertCategory(t, edit.Run(context.Background(), EditArgs{Path: "link.txt", Anchor: "outside", Replacement: "changed"}).BaseResult, ErrCategoryPathEscape)
	assertCategory(t, list.Run(context.Background(), ListArgs{Path: "outdir"}).BaseResult, ErrCategoryPathEscape)

	got, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("read outside fixture: %v", err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside file changed to %q", got)
	}
}

func TestFileopsMissingPaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	read := mustNewReadTool(t, workspace)
	write := mustNewWriteTool(t, workspace)
	edit := mustNewEditTool(t, workspace)
	list := mustNewListTool(t, workspace)

	assertCategory(t, read.Run(context.Background(), ReadArgs{Path: "missing.txt"}).BaseResult, ErrCategoryNotFound)
	assertCategory(t, write.Run(context.Background(), WriteArgs{Path: "missing/out.txt", Content: "x"}).BaseResult, ErrCategoryNotFound)
	assertCategory(t, edit.Run(context.Background(), EditArgs{Path: "missing.txt", Anchor: "a", Replacement: "b"}).BaseResult, ErrCategoryNotFound)
	assertCategory(t, list.Run(context.Background(), ListArgs{Path: "missing"}).BaseResult, ErrCategoryNotFound)
}

func assertCategory(t *testing.T, base BaseResult, want string) {
	t.Helper()

	if base.Outcome != result.OutcomeFailed {
		t.Fatalf("outcome = %q, want failed", base.Outcome)
	}
	if base.Error == nil {
		t.Fatalf("error = nil, want category %q", want)
	}
	if base.Error.Category != want {
		t.Fatalf("category = %q, want %q; message=%q", base.Error.Category, want, base.Error.Message)
	}
}

func mustNewReadTool(t *testing.T, workspace string) *ReadTool {
	t.Helper()

	tool, err := NewReadTool(workspace)
	if err != nil {
		t.Fatalf("NewReadTool: %v", err)
	}
	return tool
}

func mustNewWriteTool(t *testing.T, workspace string) *WriteTool {
	t.Helper()

	tool, err := NewWriteTool(workspace)
	if err != nil {
		t.Fatalf("NewWriteTool: %v", err)
	}
	return tool
}

func mustNewEditTool(t *testing.T, workspace string) *EditTool {
	t.Helper()

	tool, err := NewEditTool(workspace)
	if err != nil {
		t.Fatalf("NewEditTool: %v", err)
	}
	return tool
}

func mustNewListTool(t *testing.T, workspace string) *ListTool {
	t.Helper()

	tool, err := NewListTool(workspace)
	if err != nil {
		t.Fatalf("NewListTool: %v", err)
	}
	return tool
}
