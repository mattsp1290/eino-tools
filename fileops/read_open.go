package fileops

import (
	"errors"
	"os"
	"path/filepath"
)

// openRegularRead validates the opened object, not just the earlier path stat.
// Root keeps a concurrent symlink replacement inside the workspace. On Unix,
// the nonblocking flag also prevents a replacement FIFO from waiting for a
// writer before we can reject it. Both read modes consume this same descriptor.
func openRegularRead(workspace, resolved string) (*os.File, error) {
	relative, err := filepath.Rel(workspace, resolved)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile(relative, readOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, errors.New("file_read expects a regular file")
	}
	return f, nil
}
