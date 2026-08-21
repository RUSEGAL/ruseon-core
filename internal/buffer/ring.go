package buffer

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/RUSEGAL/ruseon-core/pkg/metrics"
)

// CodecParams хранит иммутабельные параметры видеокодека (VPS, SPS, PPS).
type CodecParams struct {
	VPS []byte
	SPS []byte
	PPS []byte
}

// RingBuffer хранит последние N кадров и раздает их подписчикам (Router/Broadcaster).
type RingBuffer struct {
	mu       sync.Mutex
	frames   []*Frame
	capacity int
	head     uint64
	closed   bool

	cameraID    string
	metricDrops prometheus.Counter

	params atomic.Pointer[CodecParams]

	// Подписчики
	subs      map[*Reader]struct{}
	subsSlice []*Reader

	totalDrops atomic.Uint64
}

// NewRingBuffer создает новый буфер заданного размера.
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

// SetCameraID sets the camera ID for metrics.
func (rb *RingBuffer) SetCameraID(id string) {
	rb.cameraID = id
	rb.metricDrops = metrics.RingbufferDropsTotal.WithLabelValues(id)
}

// GetTotalDrops возвращает суммарное число дропнутых кадров за все время существования буфера.
func (rb *RingBuffer) GetTotalDrops() uint64 {
	return rb.totalDrops.Load()
}

// Write добавляет новый кадр в буфер и рассылает его подписчикам без блокировок чтения подписчиков.
func (rb *RingBuffer) Write(f *Frame) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}

	// 1. Сохраняем в кольцевой буфер (для истории новым клиентам)
	// #nosec G115 -- rb.capacity is always positive
	idx := rb.head % uint64(rb.capacity)
	rb.frames[idx] = f
	rb.head++

	// 2. Рассылаем всем текущим подписчикам
	for _, sub := range rb.subsSlice {
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

// SetParams сохраняет параметры кодека (VPS, SPS, PPS), атомарно создавая иммутабельную структуру.
func (rb *RingBuffer) SetParams(vps, sps, pps []byte) {
	p := &CodecParams{
		VPS: cloneBytes(vps),
		SPS: cloneBytes(sps),
		PPS: cloneBytes(pps),
	}
	rb.params.Store(p)
}

// GetCodecParams возвращает иммутабельный указатель на текущие параметры кодека (Zero-Alloc, Zero-Lock, Zero-Copy).
// Используется во внутренних высоконагруженных hot-path (WebRTC, gRPC, HLS), где слайсы только читаются.
func (rb *RingBuffer) GetCodecParams() *CodecParams {
	return rb.params.Load()
}

// GetParams возвращает защитные копии текущих параметров кодека (VPS, SPS, PPS).
// Гарантирует обратную совместимость и изоляцию от мутаций вызывающего кода.
func (rb *RingBuffer) GetParams() ([]byte, []byte, []byte) {
	p := rb.params.Load()
	if p == nil {
		return nil, nil, nil
	}
	return cloneBytes(p.VPS), cloneBytes(p.SPS), cloneBytes(p.PPS)
}

// Close закрывает буфер и каналы всех читателей.
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

// Close отписывает читателя от рассылки.
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
