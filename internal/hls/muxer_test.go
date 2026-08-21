package hls

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
			Name:     "test_1.ts",
			Duration: 2 * time.Second,
		})
		muxer.seqCount = 1
		muxer.firstSegOnce.Do(func() { close(muxer.firstSegReady) })
		muxer.mu.Unlock()
	}()
	
	playlist, ok := muxer.GetPlaylist(context.Background())
	elapsed := time.Since(start)
	
	assert.True(t, ok)
	// Мы должны были прождать как минимум 150мс
	if elapsed < 150*time.Millisecond {
		t.Errorf("GetPlaylist returned too early, elapsed: %v", elapsed)
	}
	
	if !strings.Contains(playlist, "test_1.ts") {
		t.Errorf("Playlist does not contain test_1.ts:\n%s", playlist)
	}
}

func TestMuxer_LazyGetPlaylist_Timeout(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	muxer := NewMuxer("test", rb, nil, nil)
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	playlist, ok := muxer.GetPlaylist(ctx)
	elapsed := time.Since(start)
	
	assert.False(t, ok)
	assert.Empty(t, playlist)
	if elapsed < 150*time.Millisecond {
		t.Errorf("GetPlaylist returned too early for timeout, elapsed: %v", elapsed)
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
	
	playlist, ok := muxer.GetPlaylist(context.Background())
	assert.True(t, ok)
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

func TestMuxer_BoundedStopWithIdleStream(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()

	// Create Muxer with NO frames ever written to ringBuffer
	muxer := NewMuxer("test_idle", rb, nil, nil)
	time.Sleep(20 * time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		muxer.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// Clean bounded exit
	case <-time.After(500 * time.Millisecond):
		t.Fatal("muxer.Stop() timed out on idle stream (unbounded cancellation)")
	}
}

func TestMuxer_DynamicCodecChange_Discontinuity(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()

	// Initial SPS/PPS (e.g. 1080p)
	sps1 := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps1 := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps1, pps1)

	muxer := NewMuxer("test_codec_change", rb, nil, nil)
	muxer.targetDuration = 100 * time.Millisecond
	defer muxer.Stop()

	time.Sleep(50 * time.Millisecond)

	// 1. First segment with SPS1
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  0,
		NALUs:      [][]byte{{0x65, 0x01}},
	})
	rb.Write(&buffer.Frame{
		IsKeyFrame: false,
		Timestamp:  50 * time.Millisecond,
		NALUs:      [][]byte{{0x41, 0x02}},
	})
	// Trigger first segment close
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  150 * time.Millisecond,
		NALUs:      [][]byte{{0x65, 0x03}},
	})

	time.Sleep(50 * time.Millisecond)

	// 2. Camera changes SPS/PPS on the fly (e.g. resolution change to 720p)
	sps2 := []byte{0x67, 0x42, 0x00, 0x1f, 0xf8, 0x41, 0xa2} // different profile/level
	rb.SetParams(nil, sps2, pps1)

	// Write keyframe with new SPS parameters
	rb.Write(&buffer.Frame{
		IsKeyFrame: false,
		Timestamp:  200 * time.Millisecond,
		NALUs:      [][]byte{{0x41, 0x04}},
	})
	// Trigger second segment close
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  300 * time.Millisecond,
		NALUs:      [][]byte{{0x65, 0x05}},
	})

	time.Sleep(100 * time.Millisecond)

	playlist, ok := muxer.GetPlaylist(context.Background())
	assert.True(t, ok)
	if !strings.Contains(playlist, "#EXT-X-DISCONTINUITY") {
		t.Fatalf("expected playlist to contain #EXT-X-DISCONTINUITY on codec param change, got:\n%s", playlist)
	}
}

func BenchmarkMuxer_GetPlaylist(b *testing.B) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()
	muxer := NewMuxer("bench", rb, nil, nil)
	defer muxer.Stop()
	
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
	for b.Loop() {
		_, _ = muxer.GetPlaylist(context.Background())
	}
}

func BenchmarkMuxer_GetSegment(b *testing.B) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()
	muxer := NewMuxer("bench", rb, nil, nil)
	defer muxer.Stop()
	
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
	for b.Loop() {
		_, _ = muxer.GetSegment("stream_1.ts")
	}
}

func TestMuxer_CheckWatchdog(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()
	muxer := NewMuxer("test_watchdog", rb, nil, nil)
	defer muxer.Stop()

	// Populate 1 segment
	muxer.mu.Lock()
	muxer.segments = append(muxer.segments, &Segment{
		Name:     "stream_1.ts",
		Duration: 2 * time.Second,
		Data:     []byte{0x47, 0x01, 0x02},
	})
	muxer.seqCount = 1
	muxer.mu.Unlock()

	// 1. Fresh time -> CheckWatchdog does nothing
	now := time.Now()
	muxer.lastFrameTime.Store(now.UnixNano())
	muxer.CheckWatchdog(now)

	muxer.mu.RLock()
	assert.Len(t, muxer.segments, 1)
	muxer.mu.RUnlock()

	// 2. Old frame time (>5s) -> CheckWatchdog appends discontinuity segment
	oldTime := now.Add(-6 * time.Second)
	muxer.lastFrameTime.Store(oldTime.UnixNano())
	muxer.CheckWatchdog(now)

	muxer.mu.RLock()
	assert.Len(t, muxer.segments, 2)
	assert.True(t, muxer.segments[1].IsDiscontinuity)
	assert.Equal(t, "stream_2.ts", muxer.segments[1].Name)
	muxer.mu.RUnlock()
}

func TestGetPlaylistNoBlocking(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()
	muxer := NewMuxer("test_noblock", rb, nil, nil)
	defer muxer.Stop()

	// 100 concurrent callers waiting for first segment
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pl, ok := muxer.GetPlaylist(context.Background())
			assert.True(t, ok)
			assert.Contains(t, pl, "test_first.ts")
		}()
	}

	time.Sleep(50 * time.Millisecond)

	// Add segment and signal ready
	muxer.mu.Lock()
	muxer.segments = append(muxer.segments, &Segment{
		Name:     "test_first.ts",
		Duration: 2 * time.Second,
		Data:     []byte{0x47, 0x01, 0x02},
	})
	muxer.seqCount = 1
	muxer.firstSegOnce.Do(func() { close(muxer.firstSegReady) })
	muxer.mu.Unlock()

	wg.Wait()
}

func TestMuxer_Discontinuity(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()

	// Initial SPS/PPS
	sps1 := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps1 := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps1, pps1)

	muxer := NewMuxer("test_discontinuity", rb, nil, nil)
	muxer.targetDuration = 100 * time.Millisecond
	defer muxer.Stop()

	time.Sleep(50 * time.Millisecond)

	// Write first segment
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  0,
		NALUs:      [][]byte{{0x65, 0x01}},
	})
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  100 * time.Millisecond,
		NALUs:      [][]byte{{0x65, 0x02}},
	})

	time.Sleep(100 * time.Millisecond)

	// Change SPS (simulating on-the-fly resolution change or camera stream switch)
	sps2 := []byte{0x67, 0x42, 0x00, 0x1f, 0xf8, 0x41, 0xa2} // different profile/level
	rb.SetParams(nil, sps2, pps1)

	// Write keyframe with new SPS parameters
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  200 * time.Millisecond,
		NALUs:      [][]byte{{0x65, 0x05}},
	})

	time.Sleep(100 * time.Millisecond)

	playlist, ok := muxer.GetPlaylist(context.Background())
	assert.True(t, ok)
	if !strings.Contains(playlist, "#EXT-X-DISCONTINUITY") {
		t.Fatalf("expected playlist to contain #EXT-X-DISCONTINUITY on codec param change, got:\n%s", playlist)
	}
}

func TestMuxer_DiscontinuityDetection(t *testing.T) {
	rb := buffer.NewRingBuffer(10)
	muxer := NewMuxer("test", rb, nil, nil)

	// First segment
	muxer.mu.Lock()
	muxer.segments = append(muxer.segments, &Segment{
		Name:            "stream_1.ts",
		Duration:        2 * time.Second,
		IsDiscontinuity: false,
	})
	muxer.mu.Unlock()

	// Add second segment with discontinuity
	muxer.mu.Lock()
	muxer.segments = append(muxer.segments, &Segment{
		Name:            "stream_2.ts",
		Duration:        2 * time.Second,
		IsDiscontinuity: true,
	})
	muxer.mu.Unlock()

	muxer.mu.RLock()
	assert.Equal(t, 2, len(muxer.segments))
	assert.False(t, muxer.segments[0].IsDiscontinuity)
	assert.True(t, muxer.segments[1].IsDiscontinuity)
	assert.Equal(t, "stream_2.ts", muxer.segments[1].Name)
	muxer.mu.RUnlock()
}
