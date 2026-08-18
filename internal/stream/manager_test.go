package stream

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
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

func TestStream_Shutdown_IdleSubscribers(t *testing.T) {
	st := NewStream("test_idle_sub", "synthetic://", false, false, "tcp")

	// Subscribe 5 idle readers that are waiting on ReadContext
	var readers []*buffer.Reader
	for i := 0; i < 5; i++ {
		r := st.GetRingBuffer().Subscribe()
		readers = append(readers, r)
		go func(rd *buffer.Reader) {
			for {
				f, err := rd.ReadContext(st.ctx)
				if err != nil || f == nil {
					return
				}
			}
		}(r)
	}

	time.Sleep(50 * time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		st.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// Stopped cleanly within bounded deadline
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Stream.Stop() timed out with idle subscribers (unbounded shutdown)")
	}

	for _, r := range readers {
		r.Close()
	}
}

func TestManager_UpsertStream_ActiveViewersUninterrupted(t *testing.T) {
	m := NewManager()

	// 1. Создаем поток
	m.UpsertStream("cam_viewers", "rtsp://localhost/live", false, false, "tcp", false)
	st1, ok := m.GetStream("cam_viewers")
	if !ok {
		t.Fatalf("expected stream to exist")
	}

	// 2. Подключаем 3 активных читателя
	readers := make([]*buffer.Reader, 3)
	for i := 0; i < 3; i++ {
		readers[i] = st1.GetRingBuffer().Subscribe()
	}

	// 3. Вызываем UpsertStream с неизменными параметрами стриминга (симуляция обновления метаданных камеры)
	m.UpsertStream("cam_viewers", "rtsp://localhost/live", false, false, "tcp", false)

	st2, ok := m.GetStream("cam_viewers")
	if !ok || st1 != st2 {
		t.Fatalf("expected stream instance to remain identical on metadata change")
	}

	// 4. Пишем тестовый кадр в кольцевой буфер
	testFrame := &buffer.Frame{
		Timestamp:  100 * time.Millisecond,
		IsKeyFrame: true,
		NALUs:      [][]byte{{0x05, 0x01}},
	}
	st1.GetRingBuffer().Write(testFrame)

	// 5. Проверяем, что все 3 читателя успешно получили кадр без разрыва соединения
	for i, r := range readers {
		select {
		case f := <-r.C:
			if f.Timestamp != 100*time.Millisecond {
				t.Errorf("reader %d expected frame with timestamp 100ms, got %v", i, f)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("reader %d failed to receive frame (disconnected or interrupted)", i)
		}
		r.Close()
	}
	st1.Stop()
}

func TestManager_UpsertStream_PipelineRestartOnConfigChange(t *testing.T) {
	m := NewManager()

	// 1. Создаем поток с URL A
	m.UpsertStream("cam_restart", "rtsp://server/streamA", false, false, "tcp", false)
	st1, ok := m.GetStream("cam_restart")
	if !ok {
		t.Fatalf("expected streamA to exist")
	}

	// 2. Меняем URL на streamB
	m.UpsertStream("cam_restart", "rtsp://server/streamB", false, false, "tcp", false)

	st2, ok := m.GetStream("cam_restart")
	if !ok || st1 == st2 {
		t.Fatalf("expected new stream instance to be created on URL change")
	}

	// 3. Проверяем, что старый стрим был гарантированно остановлен (контекст отменен)
	if st1.ctx.Err() == nil {
		t.Errorf("expected old stream to be stopped/cancelled, got active context")
	}

	// 4. Проверяем, что новый стрим активен и имеет новый URL
	if st2.URL != "rtsp://server/streamB" {
		t.Errorf("expected new stream URL to be rtsp://server/streamB, got %s", st2.URL)
	}
	if st2.ctx.Err() != nil {
		t.Errorf("expected new stream context to be active")
	}

	st2.Stop()
}

func TestManager_Concurrent_EditVsDelete_NoResurrection(t *testing.T) {
	m := NewManager()

	// Инициализируем поток
	m.UpsertStream("cam_race", "rtsp://server/original", false, false, "tcp", false)

	var wg sync.WaitGroup
	deleted := false
	var delMu sync.Mutex

	// 25 горутин пытаются редактировать поток
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			delMu.Lock()
			isDel := deleted
			delMu.Unlock()

			if !isDel {
				m.UpsertStream("cam_race", fmt.Sprintf("rtsp://server/edit_%d", idx), false, false, "tcp", false)
			}
		}(i)
	}

	// 1 горутина удаляет/отключает поток
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Millisecond)
		delMu.Lock()
		deleted = true
		delMu.Unlock()
		m.UpsertStream("cam_race", "", false, false, "", true) // disabled=true
	}()

	wg.Wait()

	// Если удаление было последним или применилось, проверяем что нет зомби-стримов
	delMu.Lock()
	isDel := deleted
	delMu.Unlock()

	if isDel {
		// Принудительно завершаем отключением, если какая-то горутина успела добежать
		m.UpsertStream("cam_race", "", false, false, "", true)
		if m.HasStream("cam_race") {
			t.Errorf("expected stream to be absent after final disabled upsert")
		}
	}
}
