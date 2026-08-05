package localfs

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// LocalFS реализует registry.BlobStore поверх локальной файловой системы.
type LocalFS struct {
	baseDir string
}

func NewLocalFS(baseDir string) *LocalFS {
	return &LocalFS{
		baseDir: baseDir,
	}
}

func (l *LocalFS) fullPath(path string) string {
	if l.baseDir == "" {
		return path
	}
	return filepath.Join(l.baseDir, path)
}

func (l *LocalFS) Write(path string, data []byte) error {
	fp := l.fullPath(path)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0600)
}

func (l *LocalFS) Read(path string) ([]byte, error) {
	return os.ReadFile(l.fullPath(path))
}

func (l *LocalFS) Delete(path string) error {
	return os.Remove(l.fullPath(path))
}

func (l *LocalFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(l.fullPath(path))
}

func (l *LocalFS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(l.fullPath(path))
}

func (l *LocalFS) Create(path string) (registry.WriteSeekCloser, error) {
	fp := l.fullPath(path)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return nil, err
	}
	return os.Create(fp)
}

func (l *LocalFS) Open(path string) (io.ReadSeekCloser, error) {
	return os.Open(l.fullPath(path))
}

func (l *LocalFS) MkdirAll(path string) error {
	return os.MkdirAll(l.fullPath(path), 0755)
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
	return os.Rename(l.fullPath(oldPath), l.fullPath(newPath))
}
