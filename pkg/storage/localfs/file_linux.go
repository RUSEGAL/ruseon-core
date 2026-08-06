//go:build linux

package localfs

import (
	"os"

	"golang.org/x/sys/unix"
)

type FileWrapper struct {
	*os.File
}

func (fw *FileWrapper) DropCache() error {
	// Сбрасываем данные на диск перед удалением из кэша
	if err := fw.File.Sync(); err != nil {
		return err
	}
	// Приказываем ядру освободить Page Cache для этого файла
	return unix.Fadvise(int(fw.File.Fd()), 0, 0, unix.FADV_DONTNEED)
}
