package stream

import (
	"sync"
	"testing"
	"time"
)

func TestStreamManager_ReconnectStorm_Chaos(t *testing.T) {
	manager := NewManager()

	const numCameras = 500
	var wg sync.WaitGroup
	wg.Add(numCameras)

	// Шторм подключений
	for i := 0; i < numCameras; i++ {
		go func(id int) {
			defer wg.Done()
			streamID := string(rune('A' + id%26)) // reuse some IDs
			_ = manager.AddStream(streamID, "rtsp://dummy", false, true, "auto")
		}(i)
	}
	wg.Wait()

	// Шторм отключений и переподключений
	var wg2 sync.WaitGroup
	wg2.Add(numCameras * 2)

	for i := 0; i < numCameras; i++ {
		// Поток отключения
		go func(id int) {
			defer wg2.Done()
			streamID := string(rune('A' + id%26))
			time.Sleep(10 * time.Millisecond)
			manager.RemoveStream(streamID)
		}(i)

		// Поток нового подключения с тем же ID
		go func(id int) {
			defer wg2.Done()
			streamID := string(rune('A' + id%26))
			time.Sleep(20 * time.Millisecond)
			_ = manager.AddStream(streamID, "rtsp://dummy", false, true, "auto")
		}(i)
	}

	// Тест должен завершиться без дедлоков и паник
	done := make(chan struct{})
	go func() {
		wg2.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Chaos Test Passed: Survived reconnect storm without deadlocks")
	case <-time.After(5 * time.Second):
		t.Fatal("Chaos Test Failed: Deadlock detected during reconnect storm")
	}
}
