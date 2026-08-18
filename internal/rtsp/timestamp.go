package rtsp

import "sync"

// TimestampUnwrapper преобразует 32-битные циклические RTP-таймстампы (переполняющиеся каждые ~13.2 часа при 90 кГц)
// в непрерывную монотонно возрастающую 64-битную временную шкалу.
type TimestampUnwrapper struct {
	mu          sync.Mutex
	lastTS      uint32
	epoch       uint64
	initialized bool
}

// NewTimestampUnwrapper создает новый экземпляр разворачивателя таймстампов.
func NewTimestampUnwrapper() *TimestampUnwrapper {
	return &TimestampUnwrapper{}
}

// Unwrap принимает 32-битный RTP timestamp и возвращает 64-битный развернутый timestamp.
func (u *TimestampUnwrapper) Unwrap(ts uint32) uint64 {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.initialized {
		u.lastTS = ts
		u.epoch = 0
		u.initialized = true
		return uint64(ts)
	}

	// Переполнение вперед через границу 2^32-1 -> 0
	// Если новое значение меньше предыдущего более чем на половину диапазона (0x80000000)
	if ts < u.lastTS && (u.lastTS-ts) > 0x80000000 {
		u.epoch += 1 << 32
	} else if ts > u.lastTS && (ts-u.lastTS) > 0x80000000 {
		// Переполнение назад (редкий скачок часов назад через границу)
		if u.epoch >= (1 << 32) {
			u.epoch -= 1 << 32
		}
	}

	u.lastTS = ts
	return u.epoch + uint64(ts)
}
