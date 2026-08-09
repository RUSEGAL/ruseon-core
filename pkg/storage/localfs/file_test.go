package localfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

func TestFileWrapper_WriteAndClose(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_file_wrapper.txt")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(filePath)
	// We do NOT defer f.Close() here, because fw.Close() will close it.

	fw := NewFileWrapper(f)

	// Пишем небольшие куски данных (чтобы покрыть кейсы когда до chunkSize не дотягивает)
	_, err = fw.Write([]byte("hello "))
	if err != nil {
		t.Fatalf("Failed to write first chunk: %v", err)
	}

	// Пишем большой кусок данных, чтобы симулировать срабатывание скользящего окна (если это Linux)
	largeData := make([]byte, 3*1024*1024) // 3 MB
	for i := range largeData {
		largeData[i] = 'A'
	}
	
	_, err = fw.Write(largeData)
	if err != nil {
		t.Fatalf("Failed to write large chunk: %v", err)
	}

	// Проверяем DropCache (оно теперь всегда nil, оставлено для совместимости)
	err = fw.DropCache()
	if err != nil {
		t.Errorf("DropCache returned unexpected error: %v", err)
	}

	// Вызов Close должен корректно дописать хвост, сделать fsync и FADV_DONTNEED (на Linux)
	err = fw.Close()
	if err != nil {
		t.Fatalf("FileWrapper Close failed: %v", err)
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
