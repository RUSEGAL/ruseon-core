package stream

import (
	"fmt"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/v2/internal/buffer"
)

func BenchmarkStreamManager_GetStream(b *testing.B) {
	manager := NewManager()
	defer manager.Close()

	// Pre-populate 600 streams
	for i := 0; i < 600; i++ {
		id := fmt.Sprintf("cam_%03d", i)
		st := &Stream{
			ID:         id,
			ringBuffer: buffer.NewRingBuffer(100),
		}
		manager.mu.Lock()
		manager.streams[id] = st
		manager.mu.Unlock()
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			i++
			id := fmt.Sprintf("cam_%03d", i%600)
			_, _ = manager.GetStream(id)
		}
	})
}

func BenchmarkStream_WriteFrame(b *testing.B) {
	st := &Stream{
		ID:         "cam_bench",
		ringBuffer: buffer.NewRingBuffer(100),
	}
	defer st.ringBuffer.Close()

	frame := &buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  100 * time.Millisecond,
		NALUs:      [][]byte{make([]byte, 1024)},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		st.ringBuffer.Write(frame)
		st.framesReceived.Add(1)
		st.bytesReceived.Add(1024)
	}
}
