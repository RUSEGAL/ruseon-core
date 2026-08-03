package logger

import (
	stdlog "log"
	"testing"
	"time"
)

func TestLoggerAdapter(t *testing.T) {
	Init(true)
	
	// This will use our stdLogAdapter
	stdlog.Println("Test message from stdlib")
	
	// If it doesn't panic, it's generally fine.
	// We can't easily intercept zerolog's output without mocking its writer,
	// but this ensures the adapter Write method doesn't panic.
}

func TestBroadcaster(t *testing.T) {
	b := &Broadcaster{
		clients: make(map[chan []byte]struct{}),
	}
	
	// 1. Subscribe
	ch := b.Subscribe()
	if len(b.clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(b.clients))
	}
	
	// 2. Write
	msg := []byte("hello world")
	n, err := b.Write(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("expected write %d bytes, got %d", len(msg), n)
	}
	
	// 3. Receive
	select {
	case received := <-ch:
		if string(received) != string(msg) {
			t.Errorf("expected %s, got %s", string(msg), string(received))
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for message")
	}
	
	// 4. Write full channel handling
	// Fill up the channel
	for i := 0; i < 150; i++ {
		b.Write([]byte("spam"))
	}
	// It shouldn't block.
	
	// 5. Unsubscribe
	b.Unsubscribe(ch)
	if len(b.clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(b.clients))
	}
}
