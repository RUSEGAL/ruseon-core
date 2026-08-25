//go:build linux

package localfs

import (
	"os"

	"golang.org/x/sys/unix"
)

// FileWrapper wraps an *os.File on Linux to provide zero-copy, asynchronous disk chunk flushing
// and automatic OS Page Cache eviction (via sync_file_range and posix_fadvise FADV_DONTNEED)
// to prevent video recordings from filling kernel RAM buffers and causing IO stalls.
type FileWrapper struct {
	*os.File
	offset    int64
	lastSync  int64
	lastDrop  int64
	chunkSize int64
}

// NewFileWrapper creates a new Linux FileWrapper with a default chunk flushing threshold of 2MB.
func NewFileWrapper(f *os.File) *FileWrapper {
	return &FileWrapper{
		File:      f,
		chunkSize: 2 * 1024 * 1024, // 2MB chunking
	}
}

// Write writes bytes to the file and triggers asynchronous chunk synchronization and
// page cache purging once the unevicted data exceeds the 2MB chunkSize threshold.
func (fw *FileWrapper) Write(p []byte) (n int, err error) {
	n, err = fw.File.Write(p)
	if err != nil {
		return n, err
	}

	fw.offset += int64(n)

	if fw.offset-fw.lastSync >= fw.chunkSize {
		fd := int(fw.Fd())

		// Await and purge previous completed chunk (N-2)
		if fw.lastSync > fw.lastDrop {
			_ = unix.SyncFileRange(fd, fw.lastDrop, fw.lastSync-fw.lastDrop, unix.SYNC_FILE_RANGE_WAIT_BEFORE|unix.SYNC_FILE_RANGE_WRITE|unix.SYNC_FILE_RANGE_WAIT_AFTER)
			_ = unix.Fadvise(fd, fw.lastDrop, fw.lastSync-fw.lastDrop, unix.FADV_DONTNEED)
			fw.lastDrop = fw.lastSync
		}

		// Initiate non-blocking writeout of the current chunk (N-1)
		_ = unix.SyncFileRange(fd, fw.lastSync, fw.offset-fw.lastSync, unix.SYNC_FILE_RANGE_WRITE)
		fw.lastSync = fw.offset
	}

	return n, nil
}

// Close flushes any remaining buffered chunks, performs an fsync to persist file metadata,
// evicts cached file pages from RAM, and closes the file descriptor.
func (fw *FileWrapper) Close() error {
	remain := fw.offset - fw.lastDrop
	fd := int(fw.Fd())

	if remain > 0 {
		_ = unix.SyncFileRange(fd, fw.lastDrop, remain, unix.SYNC_FILE_RANGE_WAIT_BEFORE|unix.SYNC_FILE_RANGE_WRITE|unix.SYNC_FILE_RANGE_WAIT_AFTER)
	}

	// Persist file metadata (e.g. final file size) via fsync
	if err := fw.Sync(); err != nil {
		_ = fw.File.Close()
		return err
	}

	if remain > 0 {
		_ = unix.Fadvise(fd, fw.lastDrop, remain, unix.FADV_DONTNEED)
	}

	return fw.File.Close()
}

// DropCache satisfies registry.CacheDropper (page cache is purged incrementally during Write and Close).
func (fw *FileWrapper) DropCache() error {
	return nil
}
