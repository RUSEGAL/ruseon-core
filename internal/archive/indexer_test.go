package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetCameraArchive(t *testing.T) {
	tempDir := t.TempDir()
	camID := "cam1"
	camDir := filepath.Join(tempDir, camID)
	os.MkdirAll(camDir, 0755)

	// Create some files
	// 1. Ongoing file
	f1Name := "2026-07-31_15-04-05_ongoing.mp4"
	f1, _ := os.Create(filepath.Join(camDir, f1Name))
	f1.Close()

	// 2. Completed file (same day)
	f2Name := "2026-07-31_12-00-00_to_13-00-00.mp4"
	f2, _ := os.Create(filepath.Join(camDir, f2Name))
	f2.Close()

	// 3. Completed file (overnight)
	f3Name := "2026-07-31_23-00-00_to_01-00-00.mp4"
	f3, _ := os.Create(filepath.Join(camDir, f3Name))
	f3.Close()

	// 4. Invalid file
	f4Name := "invalid.txt"
	f4, _ := os.Create(filepath.Join(camDir, f4Name))
	f4.Close()

	intervals, err := GetCameraArchive(tempDir, camID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(intervals) != 3 {
		t.Fatalf("expected 3 intervals, got %d", len(intervals))
	}

	foundOngoing := false
	foundSameDay := false
	foundOvernight := false

	for _, inv := range intervals {
		if inv.Filename == f1Name {
			foundOngoing = true
			expectedStart, _ := time.ParseInLocation("2006-01-02_15-04-05", "2026-07-31_15-04-05", time.Local)
			if !inv.StartTime.Equal(expectedStart) {
				t.Errorf("ongoing start time mismatch")
			}
		} else if inv.Filename == f2Name {
			foundSameDay = true
			expectedStart, _ := time.ParseInLocation("2006-01-02_15-04-05", "2026-07-31_12-00-00", time.Local)
			if !inv.StartTime.Equal(expectedStart) {
				t.Errorf("same day start time mismatch")
			}
			expectedEnd := time.Date(2026, 7, 31, 13, 0, 0, 0, time.Local)
			if !inv.EndTime.Equal(expectedEnd) {
				t.Errorf("same day end time mismatch: got %v, expected %v", inv.EndTime, expectedEnd)
			}
		} else if inv.Filename == f3Name {
			foundOvernight = true
			expectedStart, _ := time.ParseInLocation("2006-01-02_15-04-05", "2026-07-31_23-00-00", time.Local)
			if !inv.StartTime.Equal(expectedStart) {
				t.Errorf("overnight start time mismatch")
			}
			// Should be next day
			expectedEnd := time.Date(2026, 8, 1, 1, 0, 0, 0, time.Local)
			if !inv.EndTime.Equal(expectedEnd) {
				t.Errorf("overnight end time mismatch: got %v, expected %v", inv.EndTime, expectedEnd)
			}
		}
	}

	if !foundOngoing || !foundSameDay || !foundOvernight {
		t.Errorf("missing some intervals")
	}

	// Test non-existent dir
	intervals2, err := GetCameraArchive(tempDir, "cam_not_exist")
	if err != nil {
		t.Fatalf("unexpected error for missing dir: %v", err)
	}
	if len(intervals2) != 0 {
		t.Errorf("expected 0 intervals for missing dir, got %d", len(intervals2))
	}
}
