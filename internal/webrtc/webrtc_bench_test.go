package webrtc

import (
	"testing"
	"time"
)

func BenchmarkCalculateFrameDuration(b *testing.B) {
	var lastPTS time.Duration
	pts := 40 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = calculateFrameDuration(pts, lastPTS, true)
	}
}
