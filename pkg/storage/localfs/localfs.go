package localfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

func (l *LocalFS) fullPath(p string) (string, error) {
	cleanPath := filepath.Clean(p)
	if filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "..") {
		return "", fmt.Errorf("invalid path: %s", p)
	}
	if l.baseDir == "" {
		return cleanPath, nil
	}
	return filepath.Join(l.baseDir, cleanPath), nil
}

func (l *LocalFS) Write(path string, data []byte) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0600)
}

func (l *LocalFS) Read(path string) ([]byte, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fp)
}

func (l *LocalFS) Delete(path string) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	return os.Remove(fp)
}

func (l *LocalFS) Stat(path string) (fs.FileInfo, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(fp)
}

func (l *LocalFS) ReadDir(path string) ([]fs.DirEntry, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(fp)
}

func (l *LocalFS) Create(path string) (registry.WriteSeekCloser, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(fp)
	if err != nil {
		return nil, err
	}
	return &FileWrapper{File: f}, nil
}

func (l *LocalFS) Open(path string) (io.ReadSeekCloser, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.Open(fp)
}

func (l *LocalFS) MkdirAll(path string) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(fp, 0755)
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
	oldFp, err := l.fullPath(oldPath)
	if err != nil {
		return err
	}
	newFp, err := l.fullPath(newPath)
	if err != nil {
		return err
	}
	return os.Rename(oldFp, newFp)
}
