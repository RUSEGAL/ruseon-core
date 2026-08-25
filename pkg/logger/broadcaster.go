package logger

import (
	"sync"
)

// Broadcaster distributes log messages concurrently to all active Server-Sent Events (SSE) subscribers.
//
// Thread-safety: Broadcaster is fully synchronized with an internal mutex.
// Non-blocking writes: slow subscriber channels are skipped to prevent logger stalls.
type Broadcaster struct {
	clients map[chan []byte]struct{}
	mu      sync.Mutex
}

// GlobalBroadcaster is the singleton Broadcaster instance wired into the root zerolog logger.
var GlobalBroadcaster = &Broadcaster{
	clients: make(map[chan []byte]struct{}),
}

// Write implements io.Writer so Broadcaster can be directly registered with zerolog.MultiLevelWriter.
//
// It makes a defensive copy of the byte slice p before dispatching to prevent data corruption
// if zerolog reuses write buffers across log lines.
func (b *Broadcaster) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.clients) == 0 {
		return len(p), nil
	}

	// Create a copy since p might be reused by zerolog before subscribers read it
	msg := make([]byte, len(p))
	copy(msg, p)

	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			// If a client's channel is full, we drop the message to avoid blocking the logger.
		}
	}
	return len(p), nil
}

// Subscribe registers a new subscriber channel with a buffer of 100 log messages.
// The caller must call Unsubscribe when finished to release resources.
func (b *Broadcaster) Subscribe() chan []byte {
	ch := make(chan []byte, 100)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel and closes it idempotently.
func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[ch]; ok {
		delete(b.clients, ch)
		close(ch)
	}
}
