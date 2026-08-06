package localfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

func TestFileWrapper_DropCache(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_fadvise.txt")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(filePath)
	defer f.Close()

	_, err = f.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	fw := &FileWrapper{File: f}

	// Вызов DropCache не должен приводить к ошибкам
	err = fw.DropCache()
	if err != nil {
		t.Errorf("DropCache returned unexpected error: %v", err)
	}
}

func TestLocalFS_CreateReturnsCacheDropper(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewLocalFS(tempDir)

	file, err := fs.Create("test_file.txt")
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()

	// Проверяем, что возвращаемый файл реализует интерфейс CacheDropper
	if dropper, ok := file.(registry.CacheDropper); !ok {
		t.Errorf("fs.Create() returned type %T which does not implement registry.CacheDropper", file)
	} else {
		err := dropper.DropCache()
		if err != nil {
			t.Errorf("DropCache returned unexpected error: %v", err)
		}
	}
}
