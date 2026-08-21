package hls

import (
	"context"
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
			_, _ = muxer.GetPlaylist(context.Background())
		}
	})
}

// BenchmarkAcquireSegment эмулирует Zero-Copy запрос к Muxer'у с ARC Release
func BenchmarkAcquireSegment(b *testing.B) {
	muxer := &Muxer{
		segments: []*Segment{
			{Name: "stream_1.ts", Duration: 2 * time.Second, Data: make([]byte, 1024*1024)}, // 1MB segment
		},
		segIndex: make(map[string]*Segment),
	}
	muxer.segIndex["stream_1.ts"] = muxer.segments[0]

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			seg, _ := muxer.AcquireSegment("stream_1.ts")
			if seg != nil {
				_ = seg.Data
				seg.Release()
			}
		}
	})
}

