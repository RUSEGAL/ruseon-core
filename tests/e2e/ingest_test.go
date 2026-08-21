package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/RUSEGAL/ruseon-core/internal/api"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/pkg/storage"
	"github.com/RUSEGAL/ruseon-core/pkg/storage/localfs"
)

const (
	mediamtxImage = "bluenviron/mediamtx:1.19.2"
	ffmpegImage   = "jrottenberg/ffmpeg:8-alpine"
)

func TestE2E_IngestAndHLS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	ctx := context.Background()

	// 1. Создаем изолированную сеть Docker для надежной коммуникации
	net, err := network.New(ctx, network.WithDriver("bridge"))
	if err != nil {
		t.Skipf("Skipping E2E test: Docker runtime or network unavailable: %v", err)
		return
	}
	defer func() {
		_ = net.Remove(ctx)
	}()

	// 2. Запускаем MediaMTX (RTSP сервер)
	req := testcontainers.ContainerRequest{
		Image:        mediamtxImage,
		ExposedPorts: []string{"8554/tcp"},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"mediamtx"},
		},
		WaitingFor: wait.ForListeningPort("8554/tcp").WithStartupTimeout(30 * time.Second),
	}
	mediamtx, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start mediamtx container: %v", err)
	}
	defer func() {
		_ = mediamtx.Terminate(ctx)
	}()

	// Получаем хост и порт RTSP сервера
	host, err := mediamtx.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get mediamtx host: %v", err)
	}
	port, err := mediamtx.MappedPort(ctx, "8554")
	if err != nil {
		t.Fatalf("Failed to get mediamtx port: %v", err)
	}

	rtspURL := fmt.Sprintf("rtsp://%s:%s/teststream", host, port.Port())
	t.Logf("MediaMTX RTSP server listening on: %s", rtspURL)

	// 3. Запускаем FFmpeg для публикации тестового потока (Test pattern) в MediaMTX
	ffmpegReq := testcontainers.ContainerRequest{
		Image:    ffmpegImage,
		Networks: []string{net.Name},
		Cmd: []string{
			"-re",
			"-f", "lavfi", "-i", "testsrc=size=640x480:rate=10",
			"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
			"-g", "10", "-keyint_min", "10",
			"-b:v", "500k",
			"-f", "rtsp", "-rtsp_transport", "tcp",
			"rtsp://mediamtx:8554/teststream",
		},
	}
	ffmpeg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: ffmpegReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start ffmpeg container: %v", err)
	}
	defer func() {
		_ = ffmpeg.Terminate(ctx)
	}()

	// Даем FFmpeg немного времени на старт публикации
	time.Sleep(3 * time.Second)

	// 4. Запускаем RUSEON Stream Manager (Ingest)
	manager := stream.NewManager()

	// Подключаем камеру (RTSP поток от MediaMTX)
	err = manager.AddStream("e2e-cam", rtspURL, false, false, "tcp", false)
	if err != nil {
		t.Fatalf("Failed to add stream: %v", err)
	}
	defer manager.RemoveStream("e2e-cam")

	// Проверяем, что Ingest работает (кадры поступают в RingBuffer)
	st, exists := manager.GetStream("e2e-cam")
	if !exists {
		t.Fatal("Stream should exist in StreamManager")
	}

	reader := st.GetRingBuffer().Subscribe()
	defer reader.Close()

	// Ожидаем получение хотя бы нескольких кадров
	framesReceived := 0
	timeout := time.After(15 * time.Second)

	for framesReceived < 5 {
		select {
		case frame := <-reader.C:
			if frame != nil {
				framesReceived++
				t.Logf("Received ingested frame %d: IsKeyFrame=%v, size=%d bytes",
					framesReceived, frame.IsKeyFrame, len(frame.NALUs))
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for frames. Received only %d/5", framesReceived)
		}
	}
	t.Log("Successfully verified RTSP Ingest pipeline!")

	// 5. Проверяем полноценный HTTP HLS API сквозного воспроизведения
	cfg := &config.Config{}
	cfg.Auth.Secret = "e2e-secret-key-32-chars-length!!"
	store, err := storage.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to initialize Badger storage: %v", err)
	}
	defer store.Close()
	registry.CurrentStateStore = store
	registry.CurrentBlobStore = localfs.NewLocalFS(t.TempDir())

	authenticator := auth.NewLocalAuthenticator(cfg)
	handler := api.NewHandler(manager, cfg, store)
	router := api.SetupRouter(handler, authenticator, false, nil)

	tsServer := httptest.NewServer(router)
	defer tsServer.Close()

	client := tsServer.Client()

	// 5.1 Проверка /livez и /readyz
	resp, err := client.Get(tsServer.URL + "/livez")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get /livez: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = client.Get(tsServer.URL + "/readyz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get /readyz: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 5.2 Запрос HLS Master Playlist
	resp, err = client.Get(tsServer.URL + "/stream/hls/e2e-cam/index.m3u8")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get HLS master playlist: err=%v, code=%d", err, resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	masterContent := string(bodyBytes)
	if !strings.Contains(masterContent, "stream.m3u8") {
		t.Fatalf("Master playlist missing stream.m3u8 reference:\n%s", masterContent)
	}
	t.Logf("HLS Master Playlist verified:\n%s", masterContent)

	// 5.3 Ждем генерации первого видеосегмента и запрашиваем Video Playlist
	var segName string
	pollTimeout := time.After(15 * time.Second)
	for segName == "" {
		select {
		case <-pollTimeout:
			t.Fatal("Timeout waiting for HLS segment generation in stream.m3u8")
		default:
			resp, err = client.Get(tsServer.URL + "/stream/hls/e2e-cam/stream.m3u8")
			if err == nil && resp.StatusCode == http.StatusOK {
				data, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				content := string(data)
				for _, line := range strings.Split(content, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasSuffix(line, ".ts") {
						segName = line
						break
					}
				}
			}
			if segName == "" {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	t.Logf("Found generated HLS segment: %s", segName)

	// 5.4 Скачиваем видеосегмент и валидируем MPEG-TS заголовок (0x47 Sync Byte)
	resp, err = client.Get(tsServer.URL + "/stream/hls/e2e-cam/" + segName)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to download HLS segment %s: err=%v, code=%d", segName, err, resp.StatusCode)
	}
	segData, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read segment data: %v", err)
	}

	if len(segData) < 188 {
		t.Fatalf("Segment data too short: %d bytes (minimum 1 TS packet = 188 bytes)", len(segData))
	}

	if segData[0] != 0x47 {
		t.Fatalf("Invalid MPEG-TS sync byte: got 0x%02X, expected 0x47", segData[0])
	}

	t.Logf("Successfully downloaded valid MPEG-TS segment %s (%d bytes, sync byte 0x47 verified)!",
		segName, len(segData))
}
