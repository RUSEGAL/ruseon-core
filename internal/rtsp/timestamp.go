package rtsp

import (
	"math"
	"sync"
	"time"
)

// TimestampUnwrapper tracks 32-bit cyclic RTP timestamps (which wrap around every ~13.2 hours at 90 kHz)
// and transforms them into a continuous, monotonically increasing 64-bit timeline.
//
// Thread-safety: fully synchronized with an internal mutex.
type TimestampUnwrapper struct {
	mu          sync.Mutex
	lastTS      uint32
	epoch       uint64
	initialized bool
}

// NewTimestampUnwrapper allocates and returns an uninitialized TimestampUnwrapper.
func NewTimestampUnwrapper() *TimestampUnwrapper {
	return &TimestampUnwrapper{}
}

// Unwrap translates a raw 32-bit RTP timestamp into a monotonic 64-bit unwrapped timestamp,
// detecting forward and backward rollover boundaries across the 0x80000000 half-range threshold.
func (u *TimestampUnwrapper) Unwrap(ts uint32) uint64 {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.initialized {
		u.lastTS = ts
		u.epoch = 0
		u.initialized = true
		return uint64(ts)
	}

	// Forward rollover past 2^32-1 -> 0
	// Detected when new timestamp is smaller than previous by more than half the 32-bit integer range
	if ts < u.lastTS && (u.lastTS-ts) > 0x80000000 {
		u.epoch += 1 << 32
	} else if ts > u.lastTS && (ts-u.lastTS) > 0x80000000 {
		// Backward rollover (rare backward clock jump)
		if u.epoch >= (1 << 32) {
			u.epoch -= 1 << 32
		}
	}

	u.lastTS = ts
	return u.epoch + uint64(ts)
}

// RTP90kToDuration converts a 90kHz RTP timestamp (uint64) into a nanosecond-precision time.Duration.
// It avoids intermediate uint64 multiplication overflow and clamps cleanly at math.MaxInt64.
func RTP90kToDuration(ts uint64) time.Duration {
	sec := ts / 90000
	rem := ts % 90000
	const maxSec = uint64(math.MaxInt64 / int64(time.Second))
	if sec > maxSec {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(int64(sec))*time.Second + time.Duration(int64(rem))*time.Second/90000
}

// DurationTo90k converts a time.Duration into 90kHz RTP clock ticks.
// Safe for durations exceeding 3.2 million years without int64 signed integer overflow.
func DurationTo90k(d time.Duration) int64 {
	sec := int64(d / time.Second)
	rem := int64(d % time.Second)
	return sec*90000 + (rem*90000)/int64(time.Second)
}
