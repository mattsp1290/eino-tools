//go:build unix

package fileops

import (
	"os"
	"syscall"
)

const readOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK
