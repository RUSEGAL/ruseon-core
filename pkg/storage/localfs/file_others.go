//go:build !linux

package localfs

import (
	"os"
)

type FileWrapper struct {
	*os.File
}

func NewFileWrapper(f *os.File) *FileWrapper {
	return &FileWrapper{File: f}
}

func (fw *FileWrapper) DropCache() error {
	return nil
}
