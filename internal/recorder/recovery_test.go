package recorder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoverCrashedFiles(t *testing.T) {
	tempDir := t.TempDir()
	camDir := filepath.Join(tempDir, "cam1")
	
	err := os.MkdirAll(camDir, 0755)
	if err != nil {
		t.Fatalf("failed to create cam dir: %v", err)
	}

	// Create crashed file
	crashedPath := filepath.Join(camDir, "2026-07-31_15-04-05_ongoing.mp4")
	f, _ := os.Create(crashedPath)
	f.Close()

	// Change modification time to simulate a crash time
	crashTime := time.Date(2026, 7, 31, 15, 10, 30, 0, time.Local)
	os.Chtimes(crashedPath, crashTime, crashTime)

	// Create a normal file to make sure it's not touched
	normalPath := filepath.Join(camDir, "2026-07-31_12-00-00_to_13-00-00.mp4")
	f2, _ := os.Create(normalPath)
	f2.Close()

	// Run recovery
	RecoverCrashedFiles(tempDir)

	// Verify crashed file was renamed
	if _, err := os.Stat(crashedPath); !os.IsNotExist(err) {
		t.Errorf("crashed file should have been renamed")
	}

	expectedPath := filepath.Join(camDir, "2026-07-31_15-04-05_to_15-10-30.mp4")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected recovered file to exist at %s", expectedPath)
	}

	// Verify normal file still exists
	if _, err := os.Stat(normalPath); os.IsNotExist(err) {
		t.Errorf("normal file should not have been touched")
	}
}
