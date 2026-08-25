package grpc

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/RUSEGAL/ruseon-core/v2/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/v2/internal/stream"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/grpc/pb"
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

func (m *mockPushMetadataServer) SetHeader(metadata.MD) error  { return nil }
func (m *mockPushMetadataServer) SendHeader(metadata.MD) error { return nil }
func (m *mockPushMetadataServer) SetTrailer(metadata.MD)       {}

func TestServer_PushMetadata(t *testing.T) {
	manager := stream.NewManager()
	srv := NewServer(manager, nil, "", "")

	// Добавляем тестовый стрим
	manager.AddStream("cam_test", "rtsp://test", false, true, "tcp", false)

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

type mockStreamFramesServer struct {
	grpc.ServerStream
	ctx       context.Context
	req       *pb.StreamRequest
	resp      []*pb.FrameResponse
	ready     chan struct{}
	readyOnce sync.Once
	sent      chan struct{}
	sentOnce  sync.Once
}

func (m *mockStreamFramesServer) Send(resp *pb.FrameResponse) error {
	m.resp = append(m.resp, resp)
	if m.sent != nil {
		m.sentOnce.Do(func() { close(m.sent) })
	}
	return nil
}

func (m *mockStreamFramesServer) Context() context.Context {
	if m.ready != nil {
		m.readyOnce.Do(func() { close(m.ready) })
	}
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func TestServer_StreamFrames(t *testing.T) {
	manager := stream.NewManager()
	srv := NewServer(manager, nil, "", "")

	// Добавляем тестовый стрим
	manager.AddStream("cam_yolo", "rtsp://yolo", false, true, "tcp", false)
	st, exists := manager.GetStream("cam_yolo")
	if !exists {
		t.Fatal("stream not created")
	}
	// Готовим тестовый кадр (RingBuffer)
	rb := st.GetRingBuffer()
	rb.SetParams([]byte{0x01}, []byte{0x02}, []byte{0x03})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	sent := make(chan struct{})

	mockSrv := &mockStreamFramesServer{
		ctx:   ctx,
		req:   &pb.StreamRequest{CameraId: "cam_yolo"},
		ready: ready,
		sent:  sent,
	}

	done := make(chan struct{})
	go func() {
		// StreamFrames блокируется пока не прервется контекст
		err := srv.StreamFrames(mockSrv.req, mockSrv)
		if err != nil {
			t.Errorf("StreamFrames failed: %v", err)
		}
		close(done)
	}()

	// Wait until StreamFrames has created its RingBuffer reader and entered the
	// request loop. Writing before this point makes the test race subscription.
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for StreamFrames subscription")
	}

	// Пишем первый кадр
	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  0,
		NALUs:      [][]byte{{0x05, 0x06}},
	})

	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for StreamFrames response")
	}

	// Отменяем контекст, чтобы выйти из цикла в StreamFrames
	cancel()

	// Пишем второй кадр, чтобы разблокировать ожидающий reader.Read()
	rb.Write(&buffer.Frame{
		IsKeyFrame: false,
		Timestamp:  0,
		NALUs:      [][]byte{{0x00}},
	})

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for StreamFrames to exit")
	}

	if len(mockSrv.resp) == 0 {
		t.Fatal("expected to receive frames, got 0")
	}

	// Проверяем формат первого ответа
	resp := mockSrv.resp[0]
	if resp.Codec != "H265" { // т.к. vps заполнен {0x01}
		t.Fatalf("expected H265, got %v", resp.Codec)
	}
	if !resp.IsKeyframe {
		t.Fatalf("expected keyframe")
	}

	// Payload должен содержать Annex B префиксы:
	// VPS, SPS, PPS, NALU
	// 0,0,0,1,0x01 + 0,0,0,1,0x02 + 0,0,0,1,0x03 + 0,0,0,1,0x05,0x06
	expectedLen := 4 + 1 + 4 + 1 + 4 + 1 + 4 + 2
	if len(resp.Payload) != expectedLen {
		t.Fatalf("expected payload len %d, got %d", expectedLen, len(resp.Payload))
	}
}
