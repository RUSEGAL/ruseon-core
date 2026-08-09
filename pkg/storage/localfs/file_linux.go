//go:build linux

package localfs

import (
	"os"

	"golang.org/x/sys/unix"
)

type FileWrapper struct {
	*os.File
	offset    int64
	lastSync  int64
	lastDrop  int64
	chunkSize int64
}

func NewFileWrapper(f *os.File) *FileWrapper {
	return &FileWrapper{
		File:      f,
		chunkSize: 2 * 1024 * 1024, // 2MB
	}
}

func (fw *FileWrapper) Write(p []byte) (n int, err error) {
	n, err = fw.File.Write(p)
	if err != nil {
		return n, err
	}

	fw.offset += int64(n)

	if fw.offset-fw.lastSync >= fw.chunkSize {
		fd := int(fw.File.Fd())

		// Сначала дожидаемся и очищаем предыдущий чанк (N-2)
		if fw.lastSync > fw.lastDrop {
			unix.SyncFileRange(fd, fw.lastDrop, fw.lastSync-fw.lastDrop, unix.SYNC_FILE_RANGE_WAIT_BEFORE|unix.SYNC_FILE_RANGE_WRITE|unix.SYNC_FILE_RANGE_WAIT_AFTER)
			unix.Fadvise(fd, fw.lastDrop, fw.lastSync-fw.lastDrop, unix.FADV_DONTNEED)
			fw.lastDrop = fw.lastSync
		}

		// Запускаем асинхронную запись текущего чанка (N-1)
		unix.SyncFileRange(fd, fw.lastSync, fw.offset-fw.lastSync, unix.SYNC_FILE_RANGE_WRITE)
		fw.lastSync = fw.offset
	}

	return n, nil
}

func (fw *FileWrapper) Close() error {
	remain := fw.offset - fw.lastDrop
	fd := int(fw.File.Fd())

	if remain > 0 {
		unix.SyncFileRange(fd, fw.lastDrop, remain, unix.SYNC_FILE_RANGE_WAIT_BEFORE|unix.SYNC_FILE_RANGE_WRITE|unix.SYNC_FILE_RANGE_WAIT_AFTER)
	}

	// Фиксируем метаданные (размер файла) через полноценный fsync
	if err := fw.File.Sync(); err != nil {
		fw.File.Close()
		return err
	}

	if remain > 0 {
		unix.Fadvise(fd, fw.lastDrop, remain, unix.FADV_DONTNEED)
	}

	return fw.File.Close()
}

// Больше не нужен, так как очистка идет на лету
func (fw *FileWrapper) DropCache() error {
	return nil
}
