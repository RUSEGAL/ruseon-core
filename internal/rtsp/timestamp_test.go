package rtsp

import (
	"testing"
)

func TestTimestampUnwrapper_Sequential(t *testing.T) {
	u := NewTimestampUnwrapper()

	ts1 := u.Unwrap(1000)
	if ts1 != 1000 {
		t.Errorf("expected 1000, got %d", ts1)
	}

	ts2 := u.Unwrap(4600)
	if ts2 != 4600 {
		t.Errorf("expected 4600, got %d", ts2)
	}

	ts3 := u.Unwrap(8200)
	if ts3 != 8200 {
		t.Errorf("expected 8200, got %d", ts3)
	}
}

func TestTimestampUnwrapper_Rollover(t *testing.T) {
	u := NewTimestampUnwrapper()

	// Начинаем близко к границе 2^32-1 (0xFFFFFFFF = 4294967295)
	startTS := uint32(0xFFFFFF00)
	u1 := u.Unwrap(startTS)
	if u1 != uint64(startTS) {
		t.Errorf("expected %d, got %d", uint64(startTS), u1)
	}

	// Переходим через 0
	wrappedTS := uint32(0x000000FF)
	u2 := u.Unwrap(wrappedTS)

	expectedU2 := uint64(1<<32) + uint64(wrappedTS)
	if u2 != expectedU2 {
		t.Errorf("expected %d after rollover, got %d", expectedU2, u2)
	}

	// Следующий кадр
	nextTS := uint32(0x00000FFF)
	u3 := u.Unwrap(nextTS)
	expectedU3 := uint64(1<<32) + uint64(nextTS)
	if u3 != expectedU3 {
		t.Errorf("expected %d, got %d", expectedU3, u3)
	}
}

func TestTimestampUnwrapper_BFrame_SmallJitter(t *testing.T) {
	u := NewTimestampUnwrapper()

	// Небольшой откат назад (B-кадр с меньшим PTS в потоке декодирования) не должен триггерить rollover
	u.Unwrap(10000)
	u2 := u.Unwrap(9500) // -500 ticks
	if u2 != 9500 {
		t.Errorf("expected 9500 for small backward jitter, got %d", u2)
	}

	u3 := u.Unwrap(13000)
	if u3 != 13000 {
		t.Errorf("expected 13000, got %d", u3)
	}
}
