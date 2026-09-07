//go:build !unix

package fileops

import "os"

const readOpenFlags = os.O_RDONLY
