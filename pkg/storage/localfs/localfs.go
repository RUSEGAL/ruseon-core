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
	if strings.Contains(p, "\x00") {
		return "", fmt.Errorf("invalid path: contains null byte")
	}

	cleanPath := filepath.Clean(p)

	if l.baseDir == "" {
		if strings.HasPrefix(cleanPath, "..") {
			return "", fmt.Errorf("invalid path: %s", p)
		}
		return cleanPath, nil
	}

	// When baseDir is configured, reject absolute paths, rooted paths, and volume names
	if filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") || filepath.VolumeName(cleanPath) != "" {
		return "", fmt.Errorf("invalid absolute or rooted path: %s", p)
	}

	cleanBase := filepath.Clean(l.baseDir)
	target := filepath.Join(cleanBase, cleanPath)

	// Ensure target is strictly confined inside cleanBase
	rel, err := filepath.Rel(cleanBase, target)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path escapes base directory: %s", p)
	}

	return target, nil
}

func (l *LocalFS) Write(path string, data []byte) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0600) // #nosec G304
}

func (l *LocalFS) Read(path string) ([]byte, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fp) // #nosec G304
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
	if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
		return nil, err
	}
	f, err := os.Create(fp) // #nosec G304
	if err != nil {
		return nil, err
	}
	return NewFileWrapper(f), nil
}

func (l *LocalFS) Open(path string) (io.ReadSeekCloser, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.Open(fp) // #nosec G304
}

func (l *LocalFS) MkdirAll(path string) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(fp, 0750)
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
