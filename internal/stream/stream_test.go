package stream

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStream_LazyHLS(t *testing.T) {
	s := NewStream("cam1", "://invalid", false, true, "tcp", false)
	defer s.Stop()

	if s.hlsMuxer != nil {
		t.Fatalf("expected nil hlsMuxer for lazy start")
	}

	muxer := s.WakeUpHLSMuxer()
	require.NotNil(t, muxer)
	require.NotNil(t, s.hlsMuxer)

	// Fast-forward last request to trigger inactivity shutdown
	oldTime := time.Now().Add(-65 * time.Second)
	s.lastHLSRequest.Store(oldTime.UnixNano())

	// Run housekeeping tick
	s.TickHousekeeping(time.Now())

	s.muxerMu.Lock()
	assert.Nil(t, s.hlsMuxer, "expected hlsMuxer to be stopped due to inactivity")
	s.muxerMu.Unlock()
}

func TestStream_GetStats(t *testing.T) {
	s := NewStream("cam2", "://invalid", false, false, "tcp", false)
	defer s.Stop()

	s.AddBytesSent(1024)

	rb := s.GetRingBuffer()
	require.NotNil(t, rb)

	stats := s.GetStats()
	assert.Equal(t, uint64(1024), stats.BytesSent)
	assert.Equal(t, "-", stats.Codec)

	// Simulate codec detection
	rb.SetParams(nil, []byte{0x01}, []byte{0x02})
	stats2 := s.GetStats()
	assert.Equal(t, "H.264 / AVC", stats2.Codec)
}

func TestStream_TickHousekeeping_Bitrate(t *testing.T) {
	s := NewStream("cam_bitrate", "synthetic://", false, false, "tcp", false)
	defer s.Stop()

	now := time.Now()
	s.AddBytesReceived(1000)

	// First tick initializes baseline
	s.TickHousekeeping(now)

	// Simulate 1 second later with 1000 more bytes (1000 bytes * 8 / 1s = 8000 bps)
	s.AddBytesReceived(1000)
	s.TickHousekeeping(now.Add(1 * time.Second))

	stats := s.GetStats()
	assert.Equal(t, float64(8000), stats.Bitrate)
}

func TestStream_StateAndStats(t *testing.T) {
	s := NewStream("cam_state_test", "rtsp://invalid", false, false, "tcp", false)
	defer s.Stop()

	stats := s.GetStats()
	assert.True(t, stats.State == "connecting" || stats.State == "offline")

	s.reconnects.Add(10)
	stats = s.GetStats()
	assert.Equal(t, uint64(10), stats.Reconnects)
}

func TestStream_Stop_Idempotent(t *testing.T) {
	s := NewStream("cam_idempotent", "synthetic://", false, true, "tcp", false)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Stop()
		}()
	}
	wg.Wait()

	assert.Error(t, s.ctx.Err())
}

func TestStream_TokenAuth(t *testing.T) {
	s1 := NewStream("cam_auth_true", "synthetic://", false, true, "tcp", true)
	defer s1.Stop()
	assert.True(t, s1.IsTokenAuth())

	s2 := NewStream("cam_auth_false", "synthetic://", false, true, "tcp", false)
	defer s2.Stop()
	assert.False(t, s2.IsTokenAuth())
}

