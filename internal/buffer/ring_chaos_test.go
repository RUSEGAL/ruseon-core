package buffer

import (
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_Chaos(t *testing.T) {
	rb := NewRingBuffer(100)

	var writersWg sync.WaitGroup
	var readersWg sync.WaitGroup

	// 1000 concurrent writers
	for i := 0; i < 1000; i++ {
		writersWg.Add(1)
		go func(writerID int) {
			defer writersWg.Done()
			for j := 0; j < 50; j++ {
				// Mix I-frames and P-frames
				frame := &Frame{IsKeyFrame: j%5 == 0, Timestamp: time.Duration(j)}
				rb.Write(frame)
				time.Sleep(10 * time.Microsecond)
			}
		}(i)
	}

	// 10000 concurrent readers
	for i := 0; i < 10000; i++ {
		readersWg.Add(1)
		go func(readerID int) {
			defer readersWg.Done()

			// Chaos: some readers subscribe and immediately close without reading
			if readerID%10 == 0 {
				r := rb.Subscribe()
				r.Close()
				return
			}

			r := rb.Subscribe()
			defer r.Close()

			count := 0
			// Read loop
			for frame := r.Read(); frame != nil; frame = r.Read() {
				count++
				// Randomly drop out early
				if count > 20 && readerID%2 == 0 {
					break
				}
			}
		}(i)
	}

	// Wait for all writers to finish their loops.
	writersWg.Wait()

	// Close the ring buffer to unblock all remaining readers waiting for data.
	rb.Close()

	// Wait for all readers to properly exit and clean up.
	readersWg.Wait()
}
