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

func TestManager_UpsertStream(t *testing.T) {
	m := NewManager()

	// 1. Создаем поток через UpsertStream
	m.UpsertStream("cam1", "rtsp://test1", false, true, "tcp", false)
	st1, ok := m.GetStream("cam1")
	if !ok || st1 == nil {
		t.Fatalf("expected cam1 to be created")
	}
	if !m.HasStream("cam1") {
		t.Errorf("expected HasStream to return true for cam1")
	}

	// 2. Идемпотентный апсерт с теми же параметрами - стрим НЕ должен пересоздаваться
	m.UpsertStream("cam1", "rtsp://test1", false, true, "tcp", false)
	st1Same, _ := m.GetStream("cam1")
	if st1 != st1Same {
		t.Errorf("expected same stream instance when parameters are identical")
	}

	// 3. Апсерт с измененным URL - стрим ДОЛЖЕН пересоздаться
	m.UpsertStream("cam1", "rtsp://test1-updated", false, true, "tcp", false)
	st1Updated, _ := m.GetStream("cam1")
	if st1 == st1Updated {
		t.Errorf("expected new stream instance when URL is updated")
	}
	if st1Updated.URL != "rtsp://test1-updated" {
		t.Errorf("expected updated URL, got %s", st1Updated.URL)
	}

	// 4. Апсерт с disabled=true - стрим ДОЛЖЕН быть остановлен и удален
	m.UpsertStream("cam1", "rtsp://test1-updated", false, true, "tcp", true)
	if m.HasStream("cam1") {
		t.Errorf("expected cam1 to be removed when disabled")
	}
	if _, ok := m.GetStream("cam1"); ok {
		t.Errorf("expected GetStream to return false for disabled cam1")
	}
}

func TestManager_UpsertConcurrency(_ *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			camID := fmt.Sprintf("cam_%d", id%5)
			for j := 0; j < 20; j++ {
				disabled := (j % 2 == 1)
				url := fmt.Sprintf("rtsp://server/%s_%d", camID, j)
				m.UpsertStream(camID, url, false, true, "tcp", disabled)
			}
		}(i)
	}
	wg.Wait()
}
