package stream

import (
	"fmt"
	"sync"
	"testing"

	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/storage"
)

func TestManager_Concurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	// Конкурентное добавление
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = m.AddStream(fmt.Sprintf("cam%d", id), "rtsp://invalid", false, true, "tcp")
		}(i)
	}
	wg.Wait()

	streams := m.GetStreams()
	if len(streams) != 100 {
		t.Errorf("Expected 100 streams, got %d", len(streams))
	}

	// Конкурентное чтение и удаление
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sid := fmt.Sprintf("cam%d", id)
			m.GetStream(sid)
			m.RemoveStream(sid)
		}(i)
	}
	wg.Wait()

	streamsAfter := m.GetStreams()
	if len(streamsAfter) != 0 {
		t.Errorf("Expected 0 streams after removal, got %d", len(streamsAfter))
	}
}

func TestManager_AddDuplicate(t *testing.T) {
	m := NewManager()
	_ = m.AddStream("cam1", "rtsp://invalid", false, true, "tcp")
	err := m.AddStream("cam1", "rtsp://invalid2", false, true, "tcp")
	if err == nil {
		t.Errorf("Expected error when adding duplicate stream")
	}
}

func TestManager_SyncWithStorage(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(tempDir)
	defer store.Close()

	store.SaveCamera(&config.CameraConfig{ID: "cam1", URL: "rtsp://test1", Disabled: false})
	store.SaveCamera(&config.CameraConfig{ID: "cam2", URL: "rtsp://test2", Disabled: true}) // should not be started

	m := NewManager()
	// Should start cam1 and NOT start cam2
	if err := m.SyncWithStorage(store); err != nil {
		t.Fatalf("failed to sync with storage: %v", err)
	}

	streams := m.GetStreams()
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream to be started, got %d", len(streams))
	}
	if streams[0].ID != "cam1" {
		t.Errorf("expected cam1, got %s", streams[0].ID)
	}

	// Now add cam3, delete cam1, enable cam2 in DB
	store.SaveCamera(&config.CameraConfig{ID: "cam3", URL: "rtsp://test3", Disabled: false})
	store.DeleteCamera("cam1")
	store.SaveCamera(&config.CameraConfig{ID: "cam2", URL: "rtsp://test2", Disabled: false})

	if err := m.SyncWithStorage(store); err != nil {
		t.Fatalf("failed to sync with storage: %v", err)
	}

	streams = m.GetStreams()
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams to be running, got %d", len(streams))
	}

	foundCam2, foundCam3 := false, false
	for _, s := range streams {
		if s.ID == "cam2" {
			foundCam2 = true
		}
		if s.ID == "cam3" {
			foundCam3 = true
		}
	}
	if !foundCam2 || !foundCam3 {
		t.Errorf("expected cam2 and cam3, got something else")
	}
}
