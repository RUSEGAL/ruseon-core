package buffer

import (
	"sync"
	"testing"
)

func BenchmarkRingBuffer_Write(b *testing.B) {
	rb := NewRingBuffer(100)
	defer rb.Close()
	frame := &Frame{IsKeyFrame: true, NALUs: [][]byte{make([]byte, 1024)}}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rb.Write(frame)
	}
}

func BenchmarkRingBuffer_WriteAndBroadcast_100_Subs(b *testing.B) {
	rb := NewRingBuffer(100)
	defer rb.Close()
	frame := &Frame{IsKeyFrame: true, NALUs: [][]byte{make([]byte, 1024)}}

	var wg sync.WaitGroup
	// Subscribe 100 readers
	for i := 0; i < 100; i++ {
		r := rb.Subscribe()
		defer r.Close()
		wg.Add(1)
		go func(reader *Reader) {
			defer wg.Done()
			for reader.Read() != nil {
				// drain the reader
			}
		}(r)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rb.Write(frame)
	}
	
	b.StopTimer()
	rb.Close()
	wg.Wait()
}
