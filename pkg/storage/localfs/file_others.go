//go:build !linux

package localfs

import (
	"os"
)

type FileWrapper struct {
	*os.File
}

func (fw *FileWrapper) DropCache() error {
	// POSIX_FADV_DONTNEED поддерживается только на Linux.
	// На других платформах просто сбрасываем данные на диск.
	return fw.File.Sync()
}
