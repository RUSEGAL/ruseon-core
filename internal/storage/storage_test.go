package storage

import (
	"testing"
	"gritprofmediaserver/internal/config"
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
