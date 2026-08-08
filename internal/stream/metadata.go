package stream

import (
	"sync"

	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

type MetadataSubscriber struct {
	C chan *pb.MetadataRequest
}

type MetadataBroadcaster struct {
	mu          sync.Mutex
	subscribers map[*MetadataSubscriber]struct{}
	latest      *pb.MetadataRequest
}

func NewMetadataBroadcaster() *MetadataBroadcaster {
	return &MetadataBroadcaster{
		subscribers: make(map[*MetadataSubscriber]struct{}),
	}
}

func (m *MetadataBroadcaster) Subscribe() *MetadataSubscriber {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	sub := &MetadataSubscriber{
		C: make(chan *pb.MetadataRequest, 10),
	}
	m.subscribers[sub] = struct{}{}
	
	// Отправляем последнее известное состояние, если оно есть и не сильно устарело
	if m.latest != nil {
		// Не блокируем подписку
		select {
		case sub.C <- m.latest:
		default:
		}
	}
	
	return sub
}

func (m *MetadataBroadcaster) Unsubscribe(sub *MetadataSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.subscribers[sub]; ok {
		delete(m.subscribers, sub)
		close(sub.C)
	}
}

func (m *MetadataBroadcaster) Broadcast(req *pb.MetadataRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.latest = req
	
	for sub := range m.subscribers {
		select {
		case sub.C <- req:
		default:
			// Дропаем метаданные, если клиент не успевает читать
		}
	}
}
