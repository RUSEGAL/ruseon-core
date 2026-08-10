package buffer

import (
	"sync/atomic"
	"unsafe"
)

// LockFreeRingBuffer is a simple MPSC (Multi-Producer, Single-Consumer) lock-free ring buffer.
type LockFreeRingBuffer[T any] struct {
	capacity uint64
	mask     uint64
	buffer   []unsafe.Pointer
	head     atomic.Uint64
	tail     atomic.Uint64
}

// NewLockFreeRingBuffer creates a new lock-free ring buffer.
// capacity must be a power of 2.
func NewLockFreeRingBuffer[T any](capacity uint64) *LockFreeRingBuffer[T] {
	// Ensure capacity is a power of 2
	capPow2 := uint64(1)
	for capPow2 < capacity {
		capPow2 <<= 1
	}
	return &LockFreeRingBuffer[T]{
		capacity: capPow2,
		mask:     capPow2 - 1,
		buffer:   make([]unsafe.Pointer, capPow2),
	}
}

// Push adds an item to the buffer. If the buffer is full, it drops the oldest item (advances tail)
// to act as a ring buffer. This is safe for concurrent producers.
func (rb *LockFreeRingBuffer[T]) Push(item *T) {
	for {
		head := rb.head.Load()
		tail := rb.tail.Load()
		
		// If full, try to advance tail (drop oldest)
		if head-tail >= rb.capacity {
			rb.tail.CompareAndSwap(tail, tail+1)
			continue // Try again
		}

		// Try to claim the slot at head
		if rb.head.CompareAndSwap(head, head+1) {
			atomic.StorePointer(&rb.buffer[head&rb.mask], unsafe.Pointer(item))
			return
		}
	}
}

// Pop removes and returns an item from the buffer. It returns nil if empty.
// This should only be called by a single consumer.
func (rb *LockFreeRingBuffer[T]) Pop() *T {
	for {
		head := rb.head.Load()
		tail := rb.tail.Load()

		if tail == head {
			return nil // Empty
		}

		ptr := atomic.LoadPointer(&rb.buffer[tail&rb.mask])
		if ptr == nil {
			// Slot claimed by producer but not yet written. Wait/retry.
			continue
		}

		// Try to advance tail
		if rb.tail.CompareAndSwap(tail, tail+1) {
			atomic.StorePointer(&rb.buffer[tail&rb.mask], nil)
			return (*T)(ptr)
		}
	}
}
