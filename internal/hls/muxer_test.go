package hls

import (
	"strings"
	"testing"
	"time"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/buffer"
)

func TestMuxer_LazyGetPlaylist_Wait(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	muxer := NewMuxer("test", rb)
	
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
	muxer := NewMuxer("test", rb)
	
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
