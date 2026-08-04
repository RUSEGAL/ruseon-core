package storage

import (
	"os"
	"testing"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
)

func TestStorage_CameraCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	cam := &config.CameraConfig{
		ID:     "cam1",
		URL:    "rtsp://test",
		Record: true,
	}

	// 1. Create (Save)
	if err := store.SaveCamera(cam); err != nil {
		t.Fatalf("failed to save camera: %v", err)
	}

	// 2. Read (Get)
	fetchedCam, err := store.GetCamera("cam1")
	if err != nil {
		t.Fatalf("failed to get camera: %v", err)
	}
	if fetchedCam.ID != "cam1" || fetchedCam.URL != "rtsp://test" || fetchedCam.Record != true {
		t.Errorf("fetched camera mismatch: %+v", fetchedCam)
	}

	// 3. Update (UpdateCameraTx)
	err = store.UpdateCameraTx("cam1", func(c *config.CameraConfig) bool {
		c.URL = "rtsp://updated"
		c.Disabled = true
		return true // trigger save
	})
	if err != nil {
		t.Fatalf("failed to update camera: %v", err)
	}

	updatedCam, err := store.GetCamera("cam1")
	if err != nil {
		t.Fatalf("failed to get updated camera: %v", err)
	}
	if updatedCam.URL != "rtsp://updated" || !updatedCam.Disabled {
		t.Errorf("camera not updated correctly: %+v", updatedCam)
	}

	// 4. List
	cams, err := store.ListCameras()
	if err != nil {
		t.Fatalf("failed to list cameras: %v", err)
	}
	if len(cams) != 1 {
		t.Errorf("expected 1 camera, got %d", len(cams))
	}
	if cams[0].ID != "cam1" {
		t.Errorf("expected cam1 in list, got %s", cams[0].ID)
	}

	// 5. Delete
	if err := store.DeleteCamera("cam1"); err != nil {
		t.Fatalf("failed to delete camera: %v", err)
	}

	// Verify deletion
	_, err = store.GetCamera("cam1")
	if err == nil {
		t.Errorf("expected error when getting deleted camera, got nil")
	}
	camsAfter, _ := store.ListCameras()
	if len(camsAfter) != 0 {
		t.Errorf("expected 0 cameras after deletion, got %d", len(camsAfter))
	}
}

func TestStorage_TagCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	tag := &config.TagConfig{
		ID:    "tag1",
		Name:  "Test Tag",
		Color: "#FF0000",
	}

	// 1. Create (Save)
	if err := store.SaveTag(tag); err != nil {
		t.Fatalf("failed to save tag: %v", err)
	}

	// 2. Read (Get)
	fetchedTag, err := store.GetTag("tag1")
	if err != nil {
		t.Fatalf("failed to get tag: %v", err)
	}
	if fetchedTag.Name != "Test Tag" || fetchedTag.Color != "#FF0000" {
		t.Errorf("fetched tag mismatch: %+v", fetchedTag)
	}

	// 3. List
	tags, err := store.ListTags()
	if err != nil {
		t.Fatalf("failed to list tags: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}

	// 4. Delete
	if err := store.DeleteTag("tag1"); err != nil {
		t.Fatalf("failed to delete tag: %v", err)
	}

	// Verify deletion
	_, err = store.GetTag("tag1")
	if err == nil {
		t.Errorf("expected error when getting deleted tag, got nil")
	}
}

func TestStorage_ExportImportJSON(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	cam := &config.CameraConfig{ID: "cam1", URL: "rtsp://test"}
	_ = store.SaveCamera(cam)
	tag := &config.TagConfig{ID: "tag1", Name: "Test"}
	_ = store.SaveTag(tag)

	// Export
	jsonBytes, err := store.ExportJSON()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Ensure exported data has some size
	if len(jsonBytes) < 10 {
		t.Fatalf("exported json is too small")
	}

	// Close old store, open a new empty one
	store.Close()

	store2Dir := t.TempDir()
	store2, _ := NewStorage(store2Dir)
	defer store2.Close()

	// Ensure empty
	cams, _ := store2.ListCameras()
	if len(cams) != 0 {
		t.Fatalf("expected empty store")
	}

	// Import
	if err := store2.ImportJSON(jsonBytes); err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	// Verify
	cams, _ = store2.ListCameras()
	if len(cams) != 1 || cams[0].ID != "cam1" {
		t.Errorf("expected cam1 to be imported, got %v", cams)
	}
	
	tags, _ := store2.ListTags()
	if len(tags) != 1 || tags[0].ID != "tag1" {
		t.Errorf("expected tag1 to be imported, got %v", tags)
	}
}

func TestStorage_BackupBadger(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewStorage(tempDir)
	defer store.Close()

	_ = store.SaveCamera(&config.CameraConfig{ID: "cam1"})

	backupPath := tempDir + "/backup.bak"
	f, err := os.Create(backupPath)
	if err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}
	defer f.Close()

	if err := store.BackupBadger(f); err != nil {
		t.Fatalf("failed to backup: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("backup file is empty")
	}
}
