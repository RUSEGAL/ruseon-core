package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
)

func createDummyMP4(t *testing.T, path string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Write Init (ftyp + moov)
	init := &fmp4.Init{
		Tracks: []*fmp4.InitTrack{{
			ID:        1,
			TimeScale: 90000,
			Codec: &fmp4.CodecH264{
				SPS: []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2},
				PPS: []byte{0x68, 0xce, 0x38, 0x80},
			},
		}},
	}
	if err := init.Marshal(f); err != nil {
		t.Fatal(err)
	}

	// Write Part 1 (moof + mdat)
	sample, _ := fmp4.NewPartSampleH26x(90000, true, [][]byte{{0x05, 0x01, 0x02}})
	part := &fmp4.Part{
		SequenceNumber: 1,
		Tracks: []*fmp4.PartTrack{{
			ID:       1,
			BaseTime: 0,
			Samples:  []*fmp4.PartSample{sample},
		}},
	}
	if err := part.Marshal(f); err != nil {
		t.Fatal(err)
	}

	// Write Part 2
	sample2, _ := fmp4.NewPartSampleH26x(90000, false, [][]byte{{0x01, 0x01, 0x02}})
	part2 := &fmp4.Part{
		SequenceNumber: 2,
		Tracks: []*fmp4.PartTrack{{
			ID:       1,
			BaseTime: 90000,
			Samples:  []*fmp4.PartSample{sample2},
		}},
	}
	if err := part2.Marshal(f); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveHLS(t *testing.T) {
	tempDir := t.TempDir()
	camDir := filepath.Join(tempDir, "cam1")
	err := os.MkdirAll(camDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	mp4Path := filepath.Join(camDir, "test.mp4")
	createDummyMP4(t, mp4Path)

	// Clear cache just in case
	ClearIndexCache()

	// Test getFileIndex
	idx, err := getFileIndex(mp4Path)
	if err != nil {
		t.Fatalf("getFileIndex failed: %v", err)
	}
	if len(idx.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(idx.Parts))
	}

	// Test GenerateHLSPlaylist
	playlist, err := GenerateHLSPlaylist(tempDir, "cam1", "test.mp4")
	if err != nil {
		t.Fatalf("GenerateHLSPlaylist failed: %v", err)
	}
	if !strings.Contains(playlist, "#EXTM3U") {
		t.Fatal("expected #EXTM3U in playlist")
	}

	// Test GenerateHLSSegment
	seg, err := GenerateHLSSegment(tempDir, "cam1", "test.mp4", 0)
	if err != nil {
		t.Fatalf("GenerateHLSSegment failed for seq 0: %v", err)
	}
	if len(seg) == 0 {
		t.Fatal("expected non-empty segment 0")
	}

	seg2, err := GenerateHLSSegment(tempDir, "cam1", "test.mp4", 1)
	if err != nil {
		t.Fatalf("GenerateHLSSegment failed for seq 1: %v", err)
	}
	if len(seg2) == 0 {
		t.Fatal("expected non-empty segment 1")
	}
	
	// Test out of bounds
	_, err = GenerateHLSSegment(tempDir, "cam1", "test.mp4", 5)
	if err == nil {
		t.Fatal("expected error for out of bounds segment")
	}
}

func TestIndexLRUCache_Operations(t *testing.T) {
	cache := NewIndexLRUCache(2)

	idx1 := &FileIndex{InitSize: 100}
	idx2 := &FileIndex{InitSize: 200}
	idx3 := &FileIndex{InitSize: 300}

	cache.Add("file1", idx1)
	cache.Add("file2", idx2)

	if cache.Len() != 2 {
		t.Fatalf("expected cache length 2, got %d", cache.Len())
	}

	// Access file1 to make file2 least recently used
	if val, ok := cache.Get("file1"); !ok || val != idx1 {
		t.Fatalf("expected file1 in cache")
	}

	// Add file3, should evict file2
	cache.Add("file3", idx3)

	if _, ok := cache.Get("file2"); ok {
		t.Fatalf("expected file2 to be evicted")
	}
	if val, ok := cache.Get("file1"); !ok || val != idx1 {
		t.Fatalf("expected file1 to remain in cache")
	}
	if val, ok := cache.Get("file3"); !ok || val != idx3 {
		t.Fatalf("expected file3 in cache")
	}

	// Remove file1
	cache.Remove("file1")
	if _, ok := cache.Get("file1"); ok {
		t.Fatalf("expected file1 to be removed")
	}
	if cache.Len() != 1 {
		t.Fatalf("expected cache length 1, got %d", cache.Len())
	}

	// Clear
	cache.Clear()
	if cache.Len() != 0 {
		t.Fatalf("expected cache length 0 after clear, got %d", cache.Len())
	}
}

func TestArchiveHLS_CorruptBoxHeader(t *testing.T) {
	tempDir := t.TempDir()
	corruptPath := filepath.Join(tempDir, "corrupt.mp4")

	// Write box header with impossible box size (> 64 MB)
	corruptData := []byte{
		0x7F, 0xFF, 0xFF, 0xFF, // huge size (2 GB)
		'm', 'o', 'o', 'f',
	}
	if err := os.WriteFile(corruptPath, corruptData, 0600); err != nil {
		t.Fatal(err)
	}

	ClearIndexCache()
	idx, err := getFileIndex(corruptPath)
	if err != nil {
		t.Fatalf("getFileIndex on corrupt file should not crash: %v", err)
	}
	// Parts should be empty because header exceeded maxBoxSize
	if len(idx.Parts) != 0 {
		t.Fatalf("expected 0 parts for corrupt box header, got %d", len(idx.Parts))
	}
}

