package localfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFS_Operations(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewLocalFS(tempDir)

	// 1. Write and Read
	testData := []byte("hello ruseon storage")
	relPath := filepath.Join("cameras", "cam1", "2026-08-18", "chunk.mp4")
	if err := fs.Write(relPath, testData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	readData, err := fs.Read(relPath)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(readData) != string(testData) {
		t.Fatalf("Read data mismatch: got %q, want %q", string(readData), string(testData))
	}

	// 2. Stat
	info, err := fs.Stat(relPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size() != int64(len(testData)) {
		t.Errorf("Stat size mismatch: got %d, want %d", info.Size(), len(testData))
	}

	// 3. ReadDir
	entries, err := fs.ReadDir(filepath.Join("cameras", "cam1", "2026-08-18"))
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "chunk.mp4" {
		t.Fatalf("ReadDir entries unexpected: %v", entries)
	}

	// 4. Create and Open
	createPath := "direct_create.bin"
	w, err := fs.Create(createPath)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := w.Write([]byte("create-content")); err != nil {
		t.Fatalf("Create Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Create Close failed: %v", err)
	}

	r, err := fs.Open(createPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	openedData, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("Open ReadAll failed: %v", err)
	}
	if string(openedData) != "create-content" {
		t.Fatalf("Open data mismatch: got %q", string(openedData))
	}

	// 5. Rename
	renamedPath := "renamed.bin"
	if err := fs.Rename(createPath, renamedPath); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if _, err := fs.Stat(createPath); err == nil {
		t.Errorf("Expected old file to not exist after rename")
	}
	if _, err := fs.Stat(renamedPath); err != nil {
		t.Fatalf("Expected renamed file to exist: %v", err)
	}

	// 6. Delete
	if err := fs.Delete(renamedPath); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := fs.Stat(renamedPath); err == nil {
		t.Errorf("Expected file to be deleted")
	}

	// 7. MkdirAll
	dirPath := filepath.Join("nested", "deep", "dir")
	if err := fs.MkdirAll(dirPath); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
}

func TestLocalFS_PathTraversalProtection(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewLocalFS(tempDir)

	// Create a canary secret file outside the storage root
	canaryDir := t.TempDir()
	canaryFile := filepath.Join(canaryDir, "secret_canary.txt")
	if err := os.WriteFile(canaryFile, []byte("SUPER_SECRET"), 0600); err != nil {
		t.Fatalf("Failed to create canary file: %v", err)
	}

	attackVectors := []struct {
		name string
		path string
	}{
		{"parent_escape_simple", "../secret.txt"},
		{"parent_escape_nested", "cameras/cam1/../../../../secret.txt"},
		{"parent_escape_deep", "a/b/c/../../../../../../etc/passwd"},
		{"unix_root_escape", "/etc/passwd"},
		{"unix_abs_path", "/var/log/syslog"},
		{"windows_backslash_root", `\Windows\System32\cmd.exe`},
		{"windows_drive_letter", `C:\Windows\System32\drivers\etc\hosts`},
		{"windows_drive_rel", `C:secret.txt`},
		{"windows_unc", `\\127.0.0.1\c$\secret.txt`},
		{"null_byte_simple", "legit_file.txt\x00/../../etc/passwd"},
		{"null_byte_in_name", "safe\x00name.mp4"},
		{"encoded_dots_literal", ".../....//secret.txt"},
	}

	for _, tc := range attackVectors {
		t.Run(tc.name, func(t *testing.T) {
			// Test fullPath directly
			fp, err := fs.fullPath(tc.path)
			if err == nil {
				t.Fatalf("Security violation: fullPath(%q) should have failed, got: %s", tc.path, fp)
			}

			// Test Read
			if _, err := fs.Read(tc.path); err == nil {
				t.Fatalf("Security violation: Read(%q) succeeded", tc.path)
			}

			// Test Write
			if err := fs.Write(tc.path, []byte("pwned")); err == nil {
				t.Fatalf("Security violation: Write(%q) succeeded", tc.path)
			}

			// Test Open
			if _, err := fs.Open(tc.path); err == nil {
				t.Fatalf("Security violation: Open(%q) succeeded", tc.path)
			}

			// Test Create
			if _, err := fs.Create(tc.path); err == nil {
				t.Fatalf("Security violation: Create(%q) succeeded", tc.path)
			}

			// Test Delete
			if err := fs.Delete(tc.path); err == nil {
				t.Fatalf("Security violation: Delete(%q) succeeded", tc.path)
			}

			// Test Stat
			if _, err := fs.Stat(tc.path); err == nil {
				t.Fatalf("Security violation: Stat(%q) succeeded", tc.path)
			}

			// Test ReadDir
			if _, err := fs.ReadDir(tc.path); err == nil {
				t.Fatalf("Security violation: ReadDir(%q) succeeded", tc.path)
			}

			// Test MkdirAll
			if err := fs.MkdirAll(tc.path); err == nil {
				t.Fatalf("Security violation: MkdirAll(%q) succeeded", tc.path)
			}

			// Test Rename as source
			if err := fs.Rename(tc.path, "safe.txt"); err == nil {
				t.Fatalf("Security violation: Rename source(%q) succeeded", tc.path)
			}

			// Test Rename as target
			if err := fs.Rename("safe.txt", tc.path); err == nil {
				t.Fatalf("Security violation: Rename target(%q) succeeded", tc.path)
			}
		})
	}
}

func TestLocalFS_ValidNestedPaths(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewLocalFS(tempDir)

	validPaths := []string{
		"file.txt",
		"sub/file.txt",
		"a/b/c/d/e/video.mp4",
		"cameras/cam_01/2026/08/18/12-00-00.ts",
		"dash-and_underscore.dat",
	}

	for _, p := range validPaths {
		t.Run(p, func(t *testing.T) {
			fp, err := fs.fullPath(p)
			if err != nil {
				t.Fatalf("fullPath(%q) failed unexpectedly: %v", p, err)
			}

			// Verify fp is prefixed by clean tempDir
			cleanBase := filepath.Clean(tempDir)
			if !strings.HasPrefix(fp, cleanBase) {
				t.Fatalf("Resolved path %q does not start with base %q", fp, cleanBase)
			}

			// Verify write and read work
			if err := fs.Write(p, []byte("data")); err != nil {
				t.Fatalf("Write(%q) failed: %v", p, err)
			}
			data, err := fs.Read(p)
			if err != nil {
				t.Fatalf("Read(%q) failed: %v", p, err)
			}
			if string(data) != "data" {
				t.Fatalf("Read mismatch for %q", p)
			}
		})
	}
}
