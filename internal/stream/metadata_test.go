package stream

import (
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

func TestMetadataBroadcaster_SubscribeReceive(t *testing.T) {
	mb := NewMetadataBroadcaster()
	sub := mb.Subscribe()

	req := &pb.MetadataRequest{CameraId: "cam1", Pts: 100}
	mb.Broadcast(req)

	select {
	case r := <-sub.C:
		if r.CameraId != "cam1" || r.Pts != 100 {
			t.Fatalf("unexpected data: %v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for metadata")
	}
}

func TestMetadataBroadcaster_Latest(t *testing.T) {
	mb := NewMetadataBroadcaster()
	
	req := &pb.MetadataRequest{CameraId: "cam1", Pts: 500}
	mb.Broadcast(req)

	sub := mb.Subscribe()
	select {
	case r := <-sub.C:
		if r.Pts != 500 {
			t.Fatalf("expected latest pts 500, got %d", r.Pts)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for latest metadata")
	}
}

func TestMetadataBroadcaster_Unsubscribe(t *testing.T) {
	mb := NewMetadataBroadcaster()
	sub := mb.Subscribe()
	mb.Unsubscribe(sub)

	_, ok := <-sub.C
	if ok {
		t.Fatal("channel should be closed after unsubscribe")
	}
}

func TestMetadataBroadcaster_SlowClient(t *testing.T) {
	mb := NewMetadataBroadcaster()
	sub := mb.Subscribe()

	for i := 0; i < 15; i++ {
		mb.Broadcast(&pb.MetadataRequest{Pts: int64(i)})
	}

	r := <-sub.C
	if r.Pts != 0 {
		t.Fatalf("expected first element to be 0, got %d", r.Pts)
	}
}

func TestMetadataBroadcaster_Race(_ *testing.T) {
	mb := NewMetadataBroadcaster()
	
	// Запускаем подписчиков
	for i := 0; i < 50; i++ {
		go func() {
			sub := mb.Subscribe()
			time.Sleep(10 * time.Millisecond)
			mb.Unsubscribe(sub)
		}()
	}

	// Запускаем бродкастеров
	for i := 0; i < 50; i++ {
		go func(n int) {
			mb.Broadcast(&pb.MetadataRequest{Pts: int64(n)})
		}(i)
	}

	time.Sleep(50 * time.Millisecond)
}
