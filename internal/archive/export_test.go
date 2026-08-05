package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/RUSEGAL/ruseon-core/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/pkg/storage/localfs"
)

func init() {
	registry.RegisterBlobStore(localfs.NewLocalFS(""))
}

func TestExportMP4(t *testing.T) {
	tempDir := t.TempDir()
	camDir := filepath.Join(tempDir, "cam1")
	err := os.MkdirAll(camDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	mp4Path := filepath.Join(camDir, "test.mp4")
	createDummyMP4(t, mp4Path) // Reusing from hls_test.go

	// Test full export (startSeq: -1, endSeq: -1 means full file)
	buf := new(bytes.Buffer)
	err = ExportMP4(tempDir, "cam1", "test.mp4", -1, -1, buf)
	if err != nil {
		t.Fatalf("ExportMP4 failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected non-empty output")
	}

	fullSize := buf.Len()

	// Test partial export (startSeq: 0, endSeq: 0)
	bufPartial := new(bytes.Buffer)
	err = ExportMP4(tempDir, "cam1", "test.mp4", 0, 0, bufPartial)
	if err != nil {
		t.Fatalf("ExportMP4 partial failed: %v", err)
	}

	if bufPartial.Len() == 0 {
		t.Fatal("expected non-empty output for partial export")
	}
	
	if bufPartial.Len() >= fullSize {
		t.Fatalf("partial export should be smaller than full export (partial: %d, full: %d)", bufPartial.Len(), fullSize)
	}
	
	// Test out of bounds
	bufOob := new(bytes.Buffer)
	err = ExportMP4(tempDir, "cam1", "test.mp4", 5, 10, bufOob)
	if err != nil {
		t.Fatalf("ExportMP4 with out of bounds bounds failed: %v", err)
	}
	
	// If startSeq > endSeq (after clamping), it clamps. 
	// Our clamping logic:
	// if endSeq >= len(idx.Parts) { endSeq = len - 1 }  (2 parts, len = 2. max endSeq = 1)
	// if startSeq > endSeq { startSeq = endSeq } (startSeq becomes 1, endSeq 1)
	// So it should export the last part.
	if bufOob.Len() == 0 {
		t.Fatal("expected clamped export to succeed")
	}
}
