package grpc

import (
	"context"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
	"google.golang.org/grpc/metadata"
)

type mockPushMetadataServer struct {
	grpc.ServerStream
	reqs []*pb.MetadataRequest
	resp *pb.MetadataResponse
}

func (m *mockPushMetadataServer) Recv() (*pb.MetadataRequest, error) {
	if len(m.reqs) == 0 {
		return nil, io.EOF
	}
	req := m.reqs[0]
	m.reqs = m.reqs[1:]
	return req, nil
}

func (m *mockPushMetadataServer) SendAndClose(resp *pb.MetadataResponse) error {
	m.resp = resp
	return nil
}

func (m *mockPushMetadataServer) Context() context.Context {
	return context.Background()
}

func (m *mockPushMetadataServer) SetHeader(metadata.MD) error { return nil }
func (m *mockPushMetadataServer) SendHeader(metadata.MD) error { return nil }
func (m *mockPushMetadataServer) SetTrailer(metadata.MD) { }

func TestServer_PushMetadata(t *testing.T) {
	manager := stream.NewManager()
	srv := NewServer(manager)

	// Добавляем тестовый стрим
	manager.AddStream("cam_test", "rtsp://test", false, true, "tcp")
	
	st, exists := manager.GetStream("cam_test")
	if !exists {
		t.Fatal("stream not created")
	}

	// Подписываемся на метаданные через Broadcaster напрямую
	sub := st.GetMetadataBroadcaster().Subscribe()

	// Имитируем входящий gRPC запрос с рамками
	mockSrv := &mockPushMetadataServer{
		reqs: []*pb.MetadataRequest{
			{
				CameraId: "cam_test",
				Pts:      12345,
				Objects: []*pb.BoundingBox{
					{Label: "person", Confidence: 0.95, X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
				},
			},
		},
	}

	err := srv.PushMetadata(mockSrv)
	if err != nil {
		t.Fatalf("PushMetadata failed: %v", err)
	}

	// Проверяем ответ
	if mockSrv.resp == nil || !mockSrv.resp.Success {
		t.Fatalf("expected success response, got %v", mockSrv.resp)
	}

	// Проверяем, что метаданные дошли до подписчика
	select {
	case req := <-sub.C:
		if req.CameraId != "cam_test" {
			t.Fatalf("expected cam_test, got %v", req.CameraId)
		}
		if req.Pts != 12345 {
			t.Fatalf("expected pts 12345, got %v", req.Pts)
		}
		if len(req.Objects) != 1 || req.Objects[0].Label != "person" {
			t.Fatalf("expected 1 person object, got %v", req.Objects)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcasted metadata")
	}
}
