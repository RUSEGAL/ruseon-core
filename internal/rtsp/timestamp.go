package rtsp

import (
	"math"
	"sync"
	"time"
)

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

// RTP90kToDuration переводит 90kHz RTP timestamp (uint64) в time.Duration.
// Полностью исключает промежуточное переполнение uint64 и гарантирует точную шкалу до наносекунд.
func RTP90kToDuration(ts uint64) time.Duration {
	sec := ts / 90000
	rem := ts % 90000
	const maxSec = uint64(math.MaxInt64 / int64(time.Second))
	if sec > maxSec {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(int64(sec))*time.Second + time.Duration(int64(rem))*time.Second/90000
}

// DurationTo90k переводит time.Duration (int64) в тики 90kHz.
// Безопасный диапазон превышает 3.2 миллиона лет без риска знакового переполнения int64.
func DurationTo90k(d time.Duration) int64 {
	sec := int64(d / time.Second)
	rem := int64(d % time.Second)
	return sec*90000 + (rem*90000)/int64(time.Second)
}
