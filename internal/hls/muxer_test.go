package hls

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"github.com/RUSEGAL/ruseon-core/internal/buffer"
)

func TestMuxer_LazyGetPlaylist_Wait(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	muxer := NewMuxer("test", rb, nil, nil)
	
	start := time.Now()
	
	// Эмулируем, что сегмент сгенерируется через 200мс
	go func() {
		time.Sleep(200 * time.Millisecond)
		muxer.mu.Lock()
		muxer.segments = append(muxer.segments, &Segment{
			Name: "test_1.ts",
			Duration: 2 * time.Second,
		})
		muxer.seqCount = 1
		muxer.mu.Unlock()
	}()
	
	playlist := muxer.GetPlaylist()
	elapsed := time.Since(start)
	
	// Мы должны были прождать как минимум 150мс
	if elapsed < 150*time.Millisecond {
		t.Errorf("GetPlaylist returned too early, elapsed: %v", elapsed)
	}
	
	if !strings.Contains(playlist, "test_1.ts") {
		t.Errorf("Playlist does not contain test_1.ts:\n%s", playlist)
	}
}

func TestMuxer_LazyGetPlaylist_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	rb := buffer.NewRingBuffer(10)
	muxer := NewMuxer("test", rb, nil, nil)
	
	// Ничего не генерируем. Muxer должен прождать ~3 секунды и вернуть пустой плейлист.
	start := time.Now()
	playlist := muxer.GetPlaylist()
	elapsed := time.Since(start)
	
	if elapsed < 2500*time.Millisecond {
		t.Errorf("GetPlaylist returned too early for timeout, elapsed: %v", elapsed)
	}
	
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION") {
		t.Errorf("Playlist should have target duration even if empty")
	}
}

func TestMuxer_Lifecycle_And_GetSegment(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	// We must set some SPS/PPS so the muxer can initialize TS writer
	rb.SetParams(nil, []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}, []byte{0x68, 0xce, 0x38, 0x80})
	
	muxer := NewMuxer("test_lifecycle", rb, nil, nil)
	muxer.targetDuration = 100 * time.Millisecond // very short target for testing

	// Give the muxer goroutine time to attach its reader
	time.Sleep(50 * time.Millisecond)

	// Write 1st I-frame
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  0,
		NALUs:      [][]byte{{0x65, 0x01, 0x02, 0x03}},
	})
	
	// Write 2nd frame (P-frame)
	rb.Write(&buffer.Frame{
		IsKeyFrame: false,
		Timestamp:  50 * time.Millisecond,
		NALUs:      [][]byte{{0x41, 0x04}},
	})

	// Write 3rd frame (I-frame) to trigger segment close
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  150 * time.Millisecond,
		NALUs:      [][]byte{{0x65, 0x05}},
	})
	
	// Wait a bit for Muxer to process
	time.Sleep(100 * time.Millisecond)
	
	playlist := muxer.GetPlaylist()
	if !strings.Contains(playlist, "stream_1.ts") {
		t.Errorf("Expected playlist to contain stream_1.ts, got: %s", playlist)
	}

	seg, _ := muxer.GetSegment("stream_1.ts")
	if seg == nil {
		t.Fatalf("Expected segment stream_1.ts to be found")
	}
	if len(seg) == 0 {
		t.Fatalf("Segment is empty")
	}

	if s, _ := muxer.GetSegment("unknown.ts"); s != nil {
		t.Errorf("Expected nil for unknown segment")
	}

	muxer.Stop()
}

func BenchmarkMuxer_GetPlaylist(b *testing.B) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()
	muxer := NewMuxer("bench", rb, nil, nil)
	
	muxer.mu.Lock()
	for i := 0; i < 5; i++ {
		muxer.segments = append(muxer.segments, &Segment{
			Name:     fmt.Sprintf("stream_%d.ts", i),
			Duration: 2 * time.Second,
		})
	}
	muxer.seqCount = 5
	muxer.mu.Unlock()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = muxer.GetPlaylist()
	}
}

func BenchmarkMuxer_GetSegment(b *testing.B) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()
	muxer := NewMuxer("bench", rb, nil, nil)
	
	data := make([]byte, 1024*1024) // 1MB segment
	muxer.mu.Lock()
	muxer.segments = append(muxer.segments, &Segment{
		Name:     "stream_1.ts",
		Duration: 2 * time.Second,
		Data:     data,
	})
	muxer.seqCount = 1
	muxer.mu.Unlock()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = muxer.GetSegment("stream_1.ts")
	}
}
