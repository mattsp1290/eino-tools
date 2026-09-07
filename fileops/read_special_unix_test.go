//go:build unix

package fileops

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/mattsp1290/eino-tools/result"
)

func TestReadRejectsFIFOsInBothModes(t *testing.T) {
	workspace := t.TempDir()
	fifo := filepath.Join(workspace, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("pipe", filepath.Join(workspace, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "regular"), []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	read := mustNewReadTool(t, workspace)
	for _, name := range []string{"pipe", "link"} {
		for _, window := range []bool{false, true} {
			args := ReadArgs{Path: name}
			if window {
				offset, limit := 1, 1
				args.Offset, args.Limit = &offset, &limit
			}
			var got ReadResult
			assertFIFOOperationReturns(t, fifo, func() { got = read.Run(context.Background(), args) })
			if got.Outcome != result.OutcomeFailed || got.Error == nil || got.Content != "" {
				t.Fatalf("path=%s window=%v result=%+v", name, window, got)
			}
			args.Path = "regular"
			if next := read.Run(context.Background(), args); next.Outcome != result.OutcomeSucceeded {
				t.Fatalf("regular read after rejected FIFO: %+v", next)
			}
		}
	}
}

func TestOpenRegularReadValidatesTheOpenedDescriptor(t *testing.T) {
	workspace := canonicalizeWorkspace(t.TempDir())
	path := filepath.Join(workspace, "target")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, perr := resolveExisting(workspace, "target", false)
	if perr != nil {
		t.Fatal(perr)
	}
	if info, err := os.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("initial regular target: %v", err)
	}
	// Simulate replacement after Run's path and type checks, before opening.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	assertFIFOOperationReturns(t, path, func() {
		f, err := openRegularRead(workspace, resolved)
		if f != nil {
			_ = f.Close()
		}
		if err == nil {
			t.Error("replacement FIFO was accepted")
		}
	})
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if f, err := openRegularRead(workspace, resolved); err == nil {
		_ = f.Close()
		t.Fatal("replacement symlink escaped the workspace")
	}
}

func TestOpenRegularReadKeepsValidatedFileAfterPathReplacement(t *testing.T) {
	workspace := canonicalizeWorkspace(t.TempDir())
	path := filepath.Join(workspace, "target")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openRegularRead(workspace, path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = os.Rename(path, filepath.Join(workspace, "original")); err != nil {
		t.Fatal(err)
	}
	if err = syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "original" {
		t.Fatalf("validated descriptor data=%q err=%v", data, err)
	}
}

func assertFIFOOperationReturns(t *testing.T, fifo string, operation func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		operation()
	}()
	select {
	case <-done:
		return
	case <-time.After(time.Second):
		// Release an accidentally blocking open/read before failing the test.
		fd, err := syscall.Open(fifo, syscall.O_RDWR|syscall.O_NONBLOCK, 0)
		if err == nil {
			_ = syscall.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("FIFO operation did not exit after releasing the writer")
		}
		t.Fatal("file read blocked on a FIFO")
	}
}
