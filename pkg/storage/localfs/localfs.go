// Package localfs provides an OS filesystem implementation of registry.BlobStore
// with directory traversal sanitization, restricted file permissions, and Linux
// page cache purging optimizations.
package localfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
)

// LocalFS implements registry.BlobStore on top of the local operating system filesystem.
//
// Security guarantees:
//   - When baseDir is configured, all relative path queries are strictly verified to ensure
//     they cannot escape baseDir via directory traversal (".."), absolute paths, or volume names.
//   - Directories are created with 0750 permissions and files with 0600 permissions.
type LocalFS struct {
	baseDir string
}

// NewLocalFS creates a new LocalFS instance rooted at baseDir.
// If baseDir is empty, paths are treated relative to the working directory.
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

	// Cross-platform check: reject Windows drive letter (e.g. "C:...", "D:...")
	if len(cleanPath) >= 2 && cleanPath[1] == ':' && ((cleanPath[0] >= 'a' && cleanPath[0] <= 'z') || (cleanPath[0] >= 'A' && cleanPath[0] <= 'Z')) {
		return "", fmt.Errorf("invalid path with drive letter: %s", p)
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

// Write creates any missing parent directories and writes data to path with 0600 permissions.
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

// Read reads and returns the full content of the file at path.
func (l *LocalFS) Read(path string) ([]byte, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fp) // #nosec G304
}

// Delete removes the file or empty directory at path.
func (l *LocalFS) Delete(path string) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	return os.Remove(fp)
}

// Stat returns the fs.FileInfo describing the file at path.
func (l *LocalFS) Stat(path string) (fs.FileInfo, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(fp)
}

// ReadDir reads and returns the directory entries at path.
func (l *LocalFS) ReadDir(path string) ([]fs.DirEntry, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(fp)
}

// Create creates or truncates the named file at path, returning a registry.WriteSeekCloser wrapped with NewFileWrapper.
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

// Open opens the named file at path for reading, returning an io.ReadSeekCloser.
func (l *LocalFS) Open(path string) (io.ReadSeekCloser, error) {
	fp, err := l.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.Open(fp) // #nosec G304
}

// MkdirAll creates a directory hierarchy along with any necessary parents.
func (l *LocalFS) MkdirAll(path string) error {
	fp, err := l.fullPath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(fp, 0750)
}

// Rename moves or renames a file from oldPath to newPath.
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
