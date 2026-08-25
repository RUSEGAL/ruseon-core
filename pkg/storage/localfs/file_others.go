//go:build !linux

package localfs

import (
	"os"
)

// FileWrapper wraps an *os.File and satisfies registry.WriteSeekCloser and registry.CacheDropper
// on non-Linux platforms (Windows, macOS, BSD).
type FileWrapper struct {
	*os.File
}

// NewFileWrapper constructs a new FileWrapper around f.
func NewFileWrapper(f *os.File) *FileWrapper {
	return &FileWrapper{File: f}
}

// DropCache is a no-op on non-Linux platforms.
func (fw *FileWrapper) DropCache() error {
	return nil
}
