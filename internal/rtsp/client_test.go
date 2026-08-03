package rtsp

import (
	"context"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v4"
)

func TestNewClient_Transport(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		expected  gortsplib.Transport
	}{
		{"TCP transport", "tcp", gortsplib.TransportTCP},
		{"UDP transport", "udp", gortsplib.TransportUDP},
		{"Auto transport", "auto", gortsplib.TransportTCP},
		{"Empty transport", "", gortsplib.TransportTCP},
		{"Unknown transport", "unknown", gortsplib.TransportTCP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient("cam1", "rtsp://localhost/test", tt.transport)
			if c == nil {
				t.Fatalf("expected non-nil client")
			}
			if c.client == nil {
				t.Fatalf("expected non-nil inner client")
			}
			if c.client.Transport == nil {
				t.Fatalf("expected non-nil transport")
			}
			if *c.client.Transport != tt.expected {
				t.Errorf("expected transport %v, got %v", tt.expected, *c.client.Transport)
			}
		})
	}
}

func TestClientClose_BeforeStart(t *testing.T) {
	c := NewClient("cam1", "rtsp://localhost/test", "tcp")
	
	// Should not panic even if startDone is false
	c.Close()
	
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	
	if !closed {
		t.Errorf("expected client to be marked as closed")
	}
}

func TestClientStart_InvalidURL(t *testing.T) {
	c := NewClient("cam1", "://invalid-url", "tcp")
	
	err := c.Start(context.Background(), nil, nil)
	if err == nil {
		t.Errorf("expected error for invalid URL, got nil")
	}
}

func TestClientStart_ConnectionRefused(t *testing.T) {
	// 127.0.0.1:1 - usually connection refused
	c := NewClient("cam1", "rtsp://127.0.0.1:1/stream", "tcp")
	
	// Release the semaphore manually in case Start hangs (though it shouldn't for connection refused)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Start(ctx, nil, nil)
	if err == nil {
		t.Errorf("expected error for connection refused, got nil")
	}
}

func TestClientStart_ContextCancel(t *testing.T) {
	c := NewClient("cam1", "rtsp://127.0.0.1:1/stream", "tcp")
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := c.Start(ctx, nil, nil)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
