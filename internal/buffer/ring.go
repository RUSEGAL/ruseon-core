package buffer

import (
	"sync"
)

// RingBuffer хранит последние N кадров для обеспечения быстрого старта новых клиентов.
type RingBuffer struct {
	mu       sync.RWMutex
	cond     *sync.Cond
	frames   []*Frame
	capacity int
	head     uint64 // монотонно возрастающий индекс (текущая позиция для записи)
	closed   bool

	sps []byte
	pps []byte
}

// NewRingBuffer создает новый буфер заданного размера.
func NewRingBuffer(capacity int) *RingBuffer {
	rb := &RingBuffer{
		capacity: capacity,
		frames:   make([]*Frame, capacity),
	}
	rb.cond = sync.NewCond(rb.mu.RLocker())
	return rb
}

// Write добавляет новый кадр в буфер.
func (rb *RingBuffer) Write(f *Frame) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	idx := rb.head % uint64(rb.capacity)
	rb.frames[idx] = f
	rb.head++
	rb.cond.Broadcast()
}

// SetParams сохраняет параметры кодека (SPS/PPS).
func (rb *RingBuffer) SetParams(sps, pps []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.sps = sps
	rb.pps = pps
}

// Close закрывает буфер и будит всех читателей.
func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.closed = true
	rb.cond.Broadcast()
}

// GetParams возвращает текущие параметры кодека.
func (rb *RingBuffer) GetParams() ([]byte, []byte) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.sps, rb.pps
}

// Reader предоставляет интерфейс для чтения из RingBuffer.
type Reader struct {
	rb     *RingBuffer
	cursor uint64
}

// NewReader создает нового читателя, который начинает чтение с ближайшего прошлого ключевого кадра (I-frame).
func (rb *RingBuffer) NewReader() *Reader {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	startIdx := rb.head
	// Идем назад в поиске I-frame
	for i := 0; i < rb.capacity; i++ {
		if rb.head < uint64(i+1) {
			break
		}
		idx := rb.head - uint64(i+1)
		frame := rb.frames[idx%uint64(rb.capacity)]
		if frame != nil && frame.IsKeyFrame {
			startIdx = idx
			break
		}
	}

	return &Reader{
		rb:     rb,
		cursor: startIdx,
	}
}

// Read блокирует выполнение до появления нового кадра и возвращает его. Если буфер закрыт, возвращает nil.
func (r *Reader) Read() *Frame {
	r.rb.mu.RLock()
	defer r.rb.mu.RUnlock()

	for r.cursor >= r.rb.head {
		if r.rb.closed {
			return nil
		}
		r.rb.cond.Wait() // ожидаем поступления новых кадров
	}

	if r.rb.closed && r.cursor >= r.rb.head {
		return nil
	}

	// Проверка на overrun (писатель обогнал читателя больше чем на capacity)
	if r.rb.head-r.cursor > uint64(r.rb.capacity) {
		// Сбрасываем курсор на последний доступный I-frame, чтобы не нарушать декодирование
		r.cursor = r.rb.head - 1
		for i := 0; i < r.rb.capacity; i++ {
			idx := r.rb.head - uint64(i+1)
			if r.rb.frames[idx%uint64(r.rb.capacity)].IsKeyFrame {
				r.cursor = idx
				break
			}
		}
	}

	idx := r.cursor % uint64(r.rb.capacity)
	f := r.rb.frames[idx]
	r.cursor++
	return f
}
