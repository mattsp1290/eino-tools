package fileops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
)

func TestReadLineWindowReturnsMetadata(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "file.txt"), []byte("alpha\nbeta\ncharlie\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	offset := 2
	limit := 2
	read := mustNewReadTool(t, workspace)

	res := read.Run(context.Background(), ReadArgs{Path: "file.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if res.Content != "beta\ncharlie\n" {
		t.Fatalf("Content = %q", res.Content)
	}
	if res.NumberedContent != "2: beta\n3: charlie\n" {
		t.Fatalf("NumberedContent = %q", res.NumberedContent)
	}
	if res.LineStart != 2 || res.LineEnd != 3 || res.TotalLines != 3 {
		t.Fatalf("line metadata = start %d end %d total %d", res.LineStart, res.LineEnd, res.TotalLines)
	}
	if res.Truncated || res.NextOffset != 0 {
		t.Fatalf("Truncated=%t NextOffset=%d", res.Truncated, res.NextOffset)
	}
}

func TestReadLineWindowLimitAndEOFValidation(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "file.txt"), []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	offset := 1
	limit := 2

	res := read.Run(context.Background(), ReadArgs{Path: "file.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if !res.Truncated || res.TruncationReason != "lines" || res.NextOffset != 3 {
		t.Fatalf("truncation = %t/%q next=%d", res.Truncated, res.TruncationReason, res.NextOffset)
	}

	pastEOF := 4
	res = read.Run(context.Background(), ReadArgs{Path: "file.txt", Offset: &pastEOF, Limit: &limit})
	assertCategory(t, res.BaseResult, ErrCategoryValidation)
	if res.Error == nil || !strings.Contains(res.Error.Message, "3 total lines") {
		t.Fatalf("error = %+v, want total line count", res.Error)
	}
}

func TestReadLineWindowLongLineAndByteCaps(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	longLine := strings.Repeat("x", MaxReadLineBytes+32) + "\n"
	if err := os.WriteFile(filepath.Join(workspace, "long.txt"), []byte(longLine), 0o600); err != nil {
		t.Fatalf("write long fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	offset := 1
	limit := 1

	res := read.Run(context.Background(), ReadArgs{Path: "long.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if !res.Truncated || !res.LineTruncated || !strings.Contains(res.Content, "...(line truncated)") {
		t.Fatalf("long-line truncation not surfaced: %+v", res)
	}

	var b strings.Builder
	for i := 0; i < 4000; i++ {
		b.WriteString(strings.Repeat("a", 100))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(workspace, "many.txt"), []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write many fixture: %v", err)
	}
	limit = MaxReadWindowLines
	res = read.Run(context.Background(), ReadArgs{Path: "many.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if !res.Truncated || res.TruncationReason != "bytes" || res.NextOffset == 0 || res.ContentBytes > MaxOutputBytes {
		t.Fatalf("byte cap = truncated %t reason %q next %d bytes %d", res.Truncated, res.TruncationReason, res.NextOffset, res.ContentBytes)
	}
}

func TestReadLineWindowDoesNotReturnPartialByteCapLine(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	line := strings.Repeat("a", 100) + "\n"
	var first strings.Builder
	for first.Len()+len(line) <= MaxOutputBytes {
		first.WriteString(line)
	}
	second := strings.Repeat("b", 100) + "\n"
	if err := os.WriteFile(filepath.Join(workspace, "cap.txt"), []byte(first.String()+second), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	offset := 1
	limit := MaxReadWindowLines

	res := read.Run(context.Background(), ReadArgs{Path: "cap.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if res.Content != first.String() {
		t.Fatalf("Content contains partial second line or missed first line: len=%d", len(res.Content))
	}
	if strings.Contains(res.NumberedContent, strings.Repeat("b", 20)) {
		t.Fatalf("NumberedContent contains partial overflow line: %q", res.NumberedContent[len(res.NumberedContent)-32:])
	}
	if !res.Truncated || res.TruncationReason != "bytes" || res.LineEnd == 0 || res.NextOffset != res.LineEnd+1 {
		t.Fatalf("metadata = truncated %t reason %q lineEnd %d next %d", res.Truncated, res.TruncationReason, res.LineEnd, res.NextOffset)
	}
}

func TestReadLineWindowIgnoresSkippedLongLineTruncation(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	longLine := strings.Repeat("x", MaxReadLineBytes+32) + "\n"
	if err := os.WriteFile(filepath.Join(workspace, "skip.txt"), []byte(longLine+"selected\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	offset := 2
	limit := 1

	res := read.Run(context.Background(), ReadArgs{Path: "skip.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if res.Content != "selected\n" || res.LineTruncated || res.TruncationReason != "" || res.Truncated {
		t.Fatalf("unexpected skipped-line truncation metadata: %+v", res)
	}
}

func TestReadLineWindowBoundsVeryLongLine(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "minified.txt"), []byte(strings.Repeat("x", MaxReadLineBytes*8)), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	offset := 1
	limit := 1

	res := read.Run(context.Background(), ReadArgs{Path: "minified.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if !res.LineTruncated || !strings.Contains(res.Content, "...(line truncated)") {
		t.Fatalf("long line was not truncated: %+v", res)
	}
	if res.ContentBytes > MaxReadLineBytes+len("...(line truncated)") {
		t.Fatalf("ContentBytes = %d, want bounded line result", res.ContentBytes)
	}
}

func TestReadLineWindowCRLFBoundaryDoesNotPanic(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	line := strings.Repeat("x", MaxReadLineBytes-1) + "\r\n"
	if err := os.WriteFile(filepath.Join(workspace, "crlf.txt"), []byte(line), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	offset := 1
	limit := 1

	res := read.Run(context.Background(), ReadArgs{Path: "crlf.txt", Offset: &offset, Limit: &limit})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, err=%+v", res.Outcome, res.Error)
	}
	if res.LineTruncated || res.Content != line {
		t.Fatalf("unexpected CRLF boundary result: line_truncated=%t content_len=%d", res.LineTruncated, len(res.Content))
	}
}

func TestReadBinaryFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "bin.dat"), []byte{'t', 'e', 0, 't'}, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read := mustNewReadTool(t, workspace)
	res := read.Run(context.Background(), ReadArgs{Path: "bin.dat"})
	assertCategory(t, res.BaseResult, ErrCategoryBinary)

	offset := 1
	res = read.Run(context.Background(), ReadArgs{Path: "bin.dat", Offset: &offset})
	assertCategory(t, res.BaseResult, ErrCategoryBinary)
}
