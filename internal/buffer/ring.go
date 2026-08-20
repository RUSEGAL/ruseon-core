package buffer

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/RUSEGAL/ruseon-core/pkg/metrics"
)

// RingBuffer хранит последние N кадров и раздает их подписчикам (Router/Broadcaster).
type RingBuffer struct {
	mu       sync.RWMutex
	frames   []*Frame
	capacity int
	head     uint64 
	closed   bool

	cameraID string
	metricDrops prometheus.Counter

	vps []byte
	sps []byte
	pps []byte
	
	// Подписчики
	subMu sync.RWMutex
	subs  map[*Reader]struct{}

	totalDrops atomic.Uint64
}

// NewRingBuffer создает новый буфер заданного размера.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100
	}
	rb := &RingBuffer{
		capacity: capacity,
		frames:   make([]*Frame, capacity),
		subs:     make(map[*Reader]struct{}),
		cameraID: "unknown",
		metricDrops: metrics.RingbufferDropsTotal.WithLabelValues("unknown"),
	}
	return rb
}

// SetCameraID sets the camera ID for metrics.
func (rb *RingBuffer) SetCameraID(id string) {
	rb.cameraID = id
	rb.metricDrops = metrics.RingbufferDropsTotal.WithLabelValues(id)
}

// GetTotalDrops возвращает суммарное число дропнутых кадров за все время существования буфера.
func (rb *RingBuffer) GetTotalDrops() uint64 {
	return rb.totalDrops.Load()
}

// Write добавляет новый кадр в буфер и рассылает его подписчикам.
func (rb *RingBuffer) Write(f *Frame) {
	rb.subMu.RLock()
	defer rb.subMu.RUnlock()

	// 1. Сохраняем в кольцевой буфер (для истории новым клиентам)
	rb.mu.Lock()
	// #nosec G115 -- rb.capacity is always positive
	idx := rb.head % uint64(rb.capacity)
	rb.frames[idx] = f
	rb.head++
	rb.mu.Unlock()

	// 2. Рассылаем всем текущим подписчикам
	for sub := range rb.subs {
		if sub.NeedsIFrame.Load() {
			if !f.IsKeyFrame {
				continue // Ждем ключевой кадр после обрыва (drop)
			}
			sub.NeedsIFrame.Store(false)
		}

		// Non-blocking send
		select {
		case sub.C <- f:
			// Успешно доставили
		default:
			// Клиент тормозит, канал забит. Пропускаем кадр (Drop).
			atomic.AddUint64(&sub.Drops, 1)
			rb.metricDrops.Inc()
			rb.totalDrops.Add(1)
			sub.NeedsIFrame.Store(true) // Требуем I-Frame для возобновления
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

// SetParams сохраняет параметры кодека (VPS, SPS, PPS), создавая защитные копии срезов.
func (rb *RingBuffer) SetParams(vps, sps, pps []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.vps = cloneBytes(vps)
	rb.sps = cloneBytes(sps)
	rb.pps = cloneBytes(pps)
}

// Close закрывает буфер и каналы всех читателей.
func (rb *RingBuffer) Close() {
	rb.subMu.Lock()
	defer rb.subMu.Unlock()

	rb.closed = true
	for sub := range rb.subs {
		close(sub.C)
		delete(rb.subs, sub)
	}
}

// GetParams возвращает защитные копии текущих параметров кодека (VPS, SPS, PPS).
func (rb *RingBuffer) GetParams() ([]byte, []byte, []byte) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return cloneBytes(rb.vps), cloneBytes(rb.sps), cloneBytes(rb.pps)
}

// Reader предоставляет интерфейс для чтения из RingBuffer через неблокирующий канал.
type Reader struct {
	C           chan *Frame
	Drops       uint64
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

// Subscribe создает нового читателя. Если в истории есть кадры,
// он начинает чтение с ближайшего прошлого ключевого кадра (I-frame).
func (rb *RingBuffer) Subscribe() *Reader {
	r := &Reader{
		C:  make(chan *Frame, rb.capacity),
		rb: rb,
	}

	rb.subMu.Lock()
	defer rb.subMu.Unlock()

	if rb.closed {
		close(r.C)
		return r
	}

	rb.mu.RLock()
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
	rb.mu.RUnlock()

	rb.subs[r] = struct{}{}
	return r
}

// Close отписывает читателя от рассылки.
func (r *Reader) Close() {
	r.rb.subMu.Lock()
	defer r.rb.subMu.Unlock()
	delete(r.rb.subs, r)
	// Очищаем канал для GC, чтобы не было гонок при одновременном Write и Close
	// Сам канал закрывать здесь опасно, так как Writer может писать в него под RLock.
	// Горутина GC сама соберет канал, когда на него не останется ссылок.
}

// NewReader - обратная совместимость со старым API.
func (rb *RingBuffer) NewReader() *Reader {
	return rb.Subscribe()
}

// ReadContext возвращает следующий кадр с поддержкой отмены по контексту.
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

// Read - блокирует выполнение до получения следующего кадра или закрытия буфера.
func (r *Reader) Read() *Frame {
	f, _ := r.ReadContext(context.Background())
	return f
}
