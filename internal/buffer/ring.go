package buffer

import (
	"sync"
	"sync/atomic"
)

// RingBuffer хранит последние N кадров и раздает их подписчикам (Router/Broadcaster).
type RingBuffer struct {
	mu       sync.RWMutex
	frames   []*Frame
	capacity int
	head     uint64 
	closed   bool

	vps []byte
	sps []byte
	pps []byte
	
	// Подписчики
	subMu sync.RWMutex
	subs  map[*Reader]struct{}
}

// NewRingBuffer создает новый буфер заданного размера.
func NewRingBuffer(capacity int) *RingBuffer {
	rb := &RingBuffer{
		capacity: capacity,
		frames:   make([]*Frame, capacity),
		subs:     make(map[*Reader]struct{}),
	}
	return rb
}

// Write добавляет новый кадр в буфер и рассылает его подписчикам.
func (rb *RingBuffer) Write(f *Frame) {
	// 1. Сохраняем в кольцевой буфер (для истории новым клиентам)
	rb.mu.Lock()
	//nolint:gosec // capacity is always positive
	idx := rb.head % uint64(rb.capacity)
	rb.frames[idx] = f
	rb.head++
	rb.mu.Unlock()

	// 2. Рассылаем всем текущим подписчикам
	rb.subMu.RLock()
	defer rb.subMu.RUnlock()
	for sub := range rb.subs {
		if sub.NeedsIFrame {
			if !f.IsKeyFrame {
				continue // Ждем ключевой кадр после обрыва (drop)
			}
			sub.NeedsIFrame = false
		}

		// Non-blocking send
		select {
		case sub.C <- f:
			// Успешно доставили
		default:
			// Клиент тормозит, канал забит. Пропускаем кадр (Drop).
			atomic.AddUint64(&sub.Drops, 1)
			sub.NeedsIFrame = true // Требуем I-Frame для возобновления
		}
	}
}

// SetParams сохраняет параметры кодека (VPS, SPS, PPS).
func (rb *RingBuffer) SetParams(vps, sps, pps []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.vps = vps
	rb.sps = sps
	rb.pps = pps
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

// GetParams возвращает текущие параметры кодека (VPS, SPS, PPS).
func (rb *RingBuffer) GetParams() ([]byte, []byte, []byte) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.vps, rb.sps, rb.pps
}

// Reader предоставляет интерфейс для чтения из RingBuffer через неблокирующий канал.
type Reader struct {
	C           chan *Frame
	Drops       uint64
	NeedsIFrame bool
	rb          *RingBuffer
}

// Subscribe создает нового читателя. Если в истории есть кадры,
// он начинает чтение с ближайшего прошлого ключевого кадра (I-frame).
func (rb *RingBuffer) Subscribe() *Reader {
	// Буфер на 100 кадров позволяет компенсировать кратковременные сетевые задержки клиента
	r := &Reader{
		C:  make(chan *Frame, rb.capacity),
		rb: rb,
	}
	
	rb.mu.RLock()
	// Ищем I-Frame в истории, чтобы сразу закинуть его в канал подписчика
	startIdx := rb.head
	found := false
	for i := 0; i < rb.capacity; i++ {
		//nolint:gosec // i is always positive
		step := uint64(i + 1)
		if rb.head < step {
			break
		}
		idx := rb.head - step
		//nolint:gosec // capacity is always positive
		frame := rb.frames[idx%uint64(rb.capacity)]
		if frame != nil && frame.IsKeyFrame {
			startIdx = idx
			found = true
			break
		}
	}

	// Закидываем исторические кадры в канал (начиная с найденного I-Frame)
	if found {
		for i := startIdx; i < rb.head; i++ {
			//nolint:gosec // capacity is always positive
			f := rb.frames[i%uint64(rb.capacity)]
			if f != nil {
				r.C <- f
			}
		}
	}
	rb.mu.RUnlock()

	rb.subMu.Lock()
	if rb.closed {
		close(r.C)
	} else {
		rb.subs[r] = struct{}{}
	}
	rb.subMu.Unlock()

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

// Read - обратная совместимость со старым API.
// Блокирует выполнение до получения следующего кадра.
func (r *Reader) Read() *Frame {
	f, ok := <-r.C
	if !ok {
		return nil
	}
	return f
}
