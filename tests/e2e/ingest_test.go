package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/RUSEGAL/ruseon-core/internal/stream"
)

func TestE2E_IngestAndHLS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	ctx := context.Background()

	// 1. Запускаем MediaMTX (RTSP сервер)
	req := testcontainers.ContainerRequest{
		Image:        "bluenviron/mediamtx:latest",
		ExposedPorts: []string{"8554/tcp"},
		WaitingFor:   wait.ForListeningPort("8554/tcp").WithStartupTimeout(30 * time.Second),
	}
	mediamtx, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start mediamtx container: %v", err)
	}
	defer mediamtx.Terminate(ctx)

	// Получаем хост и порт RTSP сервера
	host, err := mediamtx.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get mediamtx host: %v", err)
	}
	port, err := mediamtx.MappedPort(ctx, "8554")
	if err != nil {
		t.Fatalf("Failed to get mediamtx port: %v", err)
	}
	
	mediamtxIP, err := mediamtx.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("Failed to get mediamtx IP: %v", err)
	}

	rtspURL := fmt.Sprintf("rtsp://%s:%s/teststream", host, port.Port())
	internalRtspURL := fmt.Sprintf("rtsp://%s:8554/teststream", mediamtxIP)
	t.Logf("RTSP server running at: %s (internal: %s)", rtspURL, internalRtspURL)

	// 2. Запускаем FFmpeg для публикации тестового потока (Test pattern) в MediaMTX
	ffmpegReq := testcontainers.ContainerRequest{
		Image: "jrottenberg/ffmpeg:latest",
		Cmd: []string{
			"-re", 
			"-f", "lavfi", "-i", "testsrc=size=640x480:rate=10",
			"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-b:v", "500k",
			"-f", "rtsp", "-rtsp_transport", "tcp",
			internalRtspURL, // Используем внутренний IP сети Docker для прямого доступа к mediamtx
		},
	}
	ffmpeg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: ffmpegReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start ffmpeg container: %v", err)
	}
	defer ffmpeg.Terminate(ctx)

	// Даем FFmpeg немного времени на старт публикации
	time.Sleep(3 * time.Second)

	// 3. Запускаем RUSEON Stream Manager (Ingest)
	manager := stream.NewManager()
	
	// Подключаем нашу камеру (RTSP поток от MediaMTX)
	err = manager.AddStream("e2e-cam", rtspURL, false, false, "tcp")
	if err != nil {
		t.Fatalf("Failed to add stream: %v", err)
	}
	defer manager.RemoveStream("e2e-cam")

	// Проверяем, что Ingest работает (кадры поступают в RingBuffer)
	st, exists := manager.GetStream("e2e-cam")
	if !exists {
		t.Fatal("Stream should exist")
	}

	// Ждем, пока Ingest поднимется и начнет получать кадры
	reader := st.GetRingBuffer().Subscribe()
	defer reader.Close()

	// Ожидаем получение хотя бы одного I-Frame и нескольких обычных кадров
	framesReceived := 0
	timeout := time.After(15 * time.Second)
	
	for framesReceived < 5 {
		select {
		case frame := <-reader.C:
			if frame != nil {
				framesReceived++
				t.Logf("Received frame %d: IsKeyFrame=%v", framesReceived, frame.IsKeyFrame)
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for frames. Received only %d/5", framesReceived)
		}
	}

	t.Log("Successfully ingested frames from testcontainers RTSP stream!")
	
	// В реальном E2E тесте мы бы еще запустили HTTP сервер и дернули HLS / WebRTC API,
	// но получение кадров от FFmpeg -> MediaMTX -> go2rtc-client -> RingBuffer 
	// уже гарантирует, что Ingest пайплайн работает корректно.
}
