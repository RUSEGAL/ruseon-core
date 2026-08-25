package buffer

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/metrics"
)

// CodecParams holds immutable video codec initialization parameters (H.264 SPS/PPS or H.265 VPS/SPS/PPS).
type CodecParams struct {
	// VPS holds the Video Parameter Set NALU (for H.265 / HEVC).
	VPS []byte
	// SPS holds the Sequence Parameter Set NALU.
	SPS []byte
	// PPS holds the Picture Parameter Set NALU.
	PPS []byte
}

// RingBuffer implements a thread-safe circular in-memory frame buffer and real-time frame broadcaster.
//
// Concurrency & Architectural design:
//   - Single-producer, multi-consumer architecture.
//   - Non-blocking broadcasts: if a subscriber's channel is saturated, frames are dropped for that
//     slow reader only, without stalling the main RTSP ingest loop or other active subscribers.
//   - Subscribers automatically synchronize to the nearest previous I-Frame on cold start, and require
//     a fresh I-Frame after experiencing frame drops.
//   - Atomic pointer storage for CodecParams allows lock-free reads on high-throughput hot paths.
type RingBuffer struct {
	mu       sync.Mutex
	frames   []*Frame
	capacity int
	head     uint64
	closed   bool

	cameraID    string
	metricDrops prometheus.Counter

	params atomic.Pointer[CodecParams]

	// Subscribers map and slice for fast iteration
	subs      map[*Reader]struct{}
	subsSlice []*Reader

	totalDrops atomic.Uint64
}

// NewRingBuffer allocates a new circular RingBuffer with the given frame capacity.
// If capacity <= 0, a default buffer size of 100 frames is used.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100
	}
	rb := &RingBuffer{
		capacity:    capacity,
		frames:      make([]*Frame, capacity),
		subs:        make(map[*Reader]struct{}),
		subsSlice:   make([]*Reader, 0),
		cameraID:    "unknown",
		metricDrops: metrics.RingbufferDropsTotal.WithLabelValues("unknown"),
	}
	return rb
}

// SetCameraID associates the buffer with a specific camera ID for Prometheus drop metric labeling.
func (rb *RingBuffer) SetCameraID(id string) {
	rb.cameraID = id
	rb.metricDrops = metrics.RingbufferDropsTotal.WithLabelValues(id)
}

// GetTotalDrops returns the total number of frames dropped across all readers since buffer creation.
func (rb *RingBuffer) GetTotalDrops() uint64 {
	return rb.totalDrops.Load()
}

// Write stores the frame in the circular history buffer and non-blockingly dispatches it to all active readers.
//
// If a reader's channel is full, the frame is dropped for that reader, its drop counter and
// Prometheus metrics are incremented, and NeedsIFrame is flagged so playback resumes cleanly on the next keyframe.
func (rb *RingBuffer) Write(f *Frame) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}

	// 1. Store into circular history buffer
	// #nosec G115 -- rb.capacity is always positive
	idx := rb.head % uint64(rb.capacity)
	rb.frames[idx] = f
	rb.head++

	// 2. Broadcast to all active subscribers
	for _, sub := range rb.subsSlice {
		if sub.NeedsIFrame.Load() {
			if !f.IsKeyFrame {
				continue // Await keyframe after drop/resync
			}
			sub.NeedsIFrame.Store(false)
		}

		// Non-blocking send
		select {
		case sub.C <- f:
			// Successfully delivered
		default:
			// Reader is lagging; drop frame and require I-frame resync
			atomic.AddUint64(&sub.Drops, 1)
			rb.metricDrops.Inc()
			rb.totalDrops.Add(1)
			sub.NeedsIFrame.Store(true)
		}
	}
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// SetParams atomically stores immutable copies of the video codec parameters (VPS, SPS, PPS).
func (rb *RingBuffer) SetParams(vps, sps, pps []byte) {
	p := &CodecParams{
		VPS: cloneBytes(vps),
		SPS: cloneBytes(sps),
		PPS: cloneBytes(pps),
	}
	rb.params.Store(p)
}

// GetCodecParams returns an immutable pointer to the current CodecParams without memory allocation or locking.
// Intended for hot paths in streaming protocols (WebRTC, gRPC, HLS).
func (rb *RingBuffer) GetCodecParams() *CodecParams {
	return rb.params.Load()
}

// GetParams returns defensive deep copies of the current VPS, SPS, and PPS byte slices.
func (rb *RingBuffer) GetParams() ([]byte, []byte, []byte) {
	p := rb.params.Load()
	if p == nil {
		return nil, nil, nil
	}
	return cloneBytes(p.VPS), cloneBytes(p.SPS), cloneBytes(p.PPS)
}

// Close closes the ring buffer and terminates all reader channels.
// Any subsequent Write or Subscribe calls are no-ops.
func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}
	rb.closed = true
	for _, sub := range rb.subsSlice {
		close(sub.C)
	}
	rb.subs = make(map[*Reader]struct{})
	rb.subsSlice = nil
}

// Reader provides a subscription handle for receiving video frames from a RingBuffer.
type Reader struct {
	// C is the non-blocking buffered receive channel of frames.
	C chan *Frame
	// Drops is the atomic count of frames dropped for this specific consumer.
	Drops uint64
	// NeedsIFrame indicates whether the reader is waiting for an IDR keyframe before accepting delta frames.
	NeedsIFrame atomic.Bool
	rb          *RingBuffer
}

func (rb *RingBuffer) findLastIFrameLocked() (uint64, bool, uint64) {
	head := rb.head
	for i := 0; i < rb.capacity; i++ {
		// #nosec G115 -- i is always non-negative
		step := uint64(i + 1)
		if head < step {
			break
		}
		idx := head - step
		// #nosec G115 -- rb.capacity is always positive
		frame := rb.frames[idx%uint64(rb.capacity)]
		if frame != nil && frame.IsKeyFrame {
			return idx, true, head
		}
	}
	return head, false, head
}

// Subscribe registers a new Reader subscriber.
// If historical frames exist in the buffer, the subscriber is preloaded starting from
// the most recent keyframe (I-frame) to ensure immediate decoder playback.
func (rb *RingBuffer) Subscribe() *Reader {
	r := &Reader{
		C:  make(chan *Frame, rb.capacity),
		rb: rb,
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		close(r.C)
		return r
	}

	startIdx, found, head := rb.findLastIFrameLocked()
	if found {
		for i := startIdx; i < head; i++ {
			// #nosec G115 -- rb.capacity is always positive
			f := rb.frames[i%uint64(rb.capacity)]
			if f != nil {
				r.C <- f
			}
		}
	} else if head > 0 {
		r.NeedsIFrame.Store(true)
	}

	rb.subs[r] = struct{}{}
	rb.subsSlice = append(rb.subsSlice, r)
	return r
}

// Close unregisters the reader and removes it from the broadcast subscriber list.
func (r *Reader) Close() {
	r.rb.mu.Lock()
	defer r.rb.mu.Unlock()

	if _, ok := r.rb.subs[r]; !ok {
		return
	}
	delete(r.rb.subs, r)
	for i, sub := range r.rb.subsSlice {
		if sub == r {
			r.rb.subsSlice = append(r.rb.subsSlice[:i], r.rb.subsSlice[i+1:]...)
			break
		}
	}
}

// NewReader is an alias for Subscribe for backwards compatibility.
func (rb *RingBuffer) NewReader() *Reader {
	return rb.Subscribe()
}

// ReadContext waits for and returns the next frame, respecting context cancellation.
// Returns io.EOF if the underlying RingBuffer is closed.
func (r *Reader) ReadContext(ctx context.Context) (*Frame, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case f, ok := <-r.C:
		if !ok {
			return nil, io.EOF
		}
		return f, nil
	}
}

// Read blocks until the next frame arrives or the buffer closes (returning nil).
func (r *Reader) Read() *Frame {
	f, _ := r.ReadContext(context.Background())
	return f
}
