package logger

import (
	"sync"
)

// Broadcaster distributes log messages to all active SSE subscribers.
type Broadcaster struct {
	clients map[chan []byte]struct{}
	mu      sync.Mutex
}

// GlobalBroadcaster is the singleton instance used by the logger.
var GlobalBroadcaster = &Broadcaster{
	clients: make(map[chan []byte]struct{}),
}

// Write implements io.Writer so it can be used with zerolog.MultiLevelWriter.
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

// Subscribe returns a channel that receives log messages.
func (b *Broadcaster) Subscribe() chan []byte {
	ch := make(chan []byte, 100)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel.
func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}
