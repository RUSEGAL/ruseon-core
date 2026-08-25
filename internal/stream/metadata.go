package stream

import (
	"sync"

	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

// MetadataSubscriber represents an individual subscriber channel for AI metadata events.
type MetadataSubscriber struct {
	// C is the buffered channel receiving AI inference metadata payloads.
	C chan *pb.MetadataRequest
}

// MetadataBroadcaster manages real-time AI metadata dispatch to subscribers (e.g. HLS WebVTT muxer, WebRTC data channels).
//
// Concurrency & Non-blocking design:
//   - Synchronized via an internal mutex.
//   - Dispatches non-blockingly: if a subscriber's channel buffer (capacity 10) is full, new metadata payloads are dropped.
type MetadataBroadcaster struct {
	mu          sync.Mutex
	subscribers map[*MetadataSubscriber]struct{}
	latest      *pb.MetadataRequest
}

// NewMetadataBroadcaster creates a new MetadataBroadcaster instance.
func NewMetadataBroadcaster() *MetadataBroadcaster {
	return &MetadataBroadcaster{
		subscribers: make(map[*MetadataSubscriber]struct{}),
	}
}

// Subscribe registers a new subscriber and immediately sends the latest cached metadata payload if available.
func (m *MetadataBroadcaster) Subscribe() *MetadataSubscriber {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	sub := &MetadataSubscriber{
		C: make(chan *pb.MetadataRequest, 10),
	}
	m.subscribers[sub] = struct{}{}
	
	// Preload latest known metadata without blocking
	if m.latest != nil {
		select {
		case sub.C <- m.latest:
		default:
		}
	}
	
	return sub
}

// Unsubscribe removes and closes the subscriber channel.
func (m *MetadataBroadcaster) Unsubscribe(sub *MetadataSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.subscribers[sub]; ok {
		delete(m.subscribers, sub)
		close(sub.C)
	}
}

// Broadcast sends the metadata payload to all registered subscribers non-blockingly.
func (m *MetadataBroadcaster) Broadcast(req *pb.MetadataRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.latest = req
	
	for sub := range m.subscribers {
		select {
		case sub.C <- req:
		default:
			// Drop metadata if consumer is lagging
		}
	}
}
