package hls

import (
	"testing"
	"time"
)

// BenchmarkGetPlaylist эмулирует конкурентные запросы к манифесту
func BenchmarkGetPlaylist(b *testing.B) {
	muxer := &Muxer{
		segments: []*Segment{
			{Name: "stream_1.ts", Duration: 2 * time.Second},
			{Name: "stream_2.ts", Duration: 2 * time.Second},
			{Name: "stream_3.ts", Duration: 2 * time.Second},
			{Name: "stream_4.ts", Duration: 2 * time.Second},
			{Name: "stream_5.ts", Duration: 2 * time.Second},
		},
		seqCount: 5,
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = muxer.GetPlaylist()
		}
	})
}

// BenchmarkGetSegment эмулирует Thundering Herd к Muxer'у
func BenchmarkGetSegment(b *testing.B) {
	muxer := &Muxer{
		segments: []*Segment{
			{Name: "stream_1.ts", Duration: 2 * time.Second, Data: make([]byte, 1024*1024)}, // 1MB segment
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = muxer.GetSegment("stream_1.ts")
		}
	})
}
