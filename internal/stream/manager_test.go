package stream

import (
	"fmt"
	"sync"
	"testing"
)

func TestManager_Concurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	// Конкурентное добавление
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = m.AddStream(fmt.Sprintf("cam%d", id), "rtsp://invalid", false, true)
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
	_ = m.AddStream("cam1", "rtsp://invalid", false, true)
	err := m.AddStream("cam1", "rtsp://invalid2", false, true)
	if err == nil {
		t.Errorf("Expected error when adding duplicate stream")
	}
}
