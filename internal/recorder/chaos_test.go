package recorder

import (
	"io"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// ChaosBlobStore эмулирует медленный или зависший диск (Throttling/Freeze).
type ChaosBlobStore struct {
	freezeDuration time.Duration
}

func (s *ChaosBlobStore) Write(_ string, _ []byte) error { return nil }
func (s *ChaosBlobStore) Read(_ string) ([]byte, error)     { return nil, nil }
func (s *ChaosBlobStore) Delete(_ string) error             { return nil }
func (s *ChaosBlobStore) Stat(_ string) (fs.FileInfo, error) { return nil, nil }
func (s *ChaosBlobStore) ReadDir(_ string) ([]fs.DirEntry, error) { return nil, nil }
func (s *ChaosBlobStore) Create(_ string) (registry.WriteSeekCloser, error) {
	time.Sleep(s.freezeDuration)
	return &discardWriteSeekCloser{Writer: io.Discard}, nil
}
func (s *ChaosBlobStore) Open(_ string) (io.ReadSeekCloser, error) { return nil, os.ErrNotExist }
func (s *ChaosBlobStore) MkdirAll(_ string) error                   { return nil }
func (s *ChaosBlobStore) Rename(_, _ string) error         { return nil }

func TestRecorder_DiskFreeze_Chaos(t *testing.T) {
	rb := buffer.NewRingBuffer(10)

	oldBlobStore := registry.CurrentBlobStore
	defer registry.RegisterBlobStore(oldBlobStore)

	// Внедряем хаос: создание файла "виснет" на 3 секунды
	registry.RegisterBlobStore(&ChaosBlobStore{freezeDuration: 3 * time.Second})

	rec := NewRecorder("chaos-stream", rb, "test_records", nil)
	defer rec.Stop()

	// Эмулируем получение параметров
	rb.SetParams(nil, []byte{0x00}, []byte{0x00})

	// Пишем кадры в буфер. Основной поток (например HLS Muxer или WebRTC)
	// НЕ ДОЛЖЕН блокироваться, даже если Recorder завис на Create()!
	start := time.Now()
	for i := 0; i < 30; i++ {
		rb.Write(&buffer.Frame{
			Timestamp:  time.Duration(i) * time.Second,
			IsKeyFrame: i%10 == 0, // Каждый 10-й кадр - ключевой (I-frame, инициирует Create)
			NALUs:      [][]byte{{0x01}},
		})
		time.Sleep(10 * time.Millisecond) // Эмуляция 100 FPS
	}
	elapsed := time.Since(start)

	// Ожидаемое время работы цикла записи в RingBuffer - около 300мс.
	// Если бы RingBuffer блокировался из-за медленного ридера (Recorder),
	// время выполнения составило бы > 3 секунд (из-за зависания Create).
	if elapsed > 1*time.Second {
		t.Fatalf("Chaos Test Failed: RingBuffer was blocked by slow Recorder. Elapsed: %v", elapsed)
	}
	t.Logf("Chaos Test Passed: RingBuffer is unblocked. Elapsed: %v", elapsed)
}
