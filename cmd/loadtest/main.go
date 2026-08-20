// Package main implements a full-stack, end-to-end load test for RUSEON Core.
//
// All layers run in-process for accurate, reproducible benchmarking:
//
//   - Ingest         : Synthetic cameras feed H.264 frames (I-frames & P-frames) directly
//                      into stream.Manager at 30 FPS.
//   - REST API       : Concurrent workers query Gin router (/api/cameras, /api/stats, /api/tags, /api/folders)
//                      exercising JWT auth, metrics aggregation and JSON serialization.
//   - HLS fMP4       : Viewers poll live playlists (/stream/hls/:id/stream.m3u8) and download
//                      generated video segments (/stream/hls/:id/segment_*.m4s & init.mp4).
//   - WebRTC (WHEP)  : Clients establish real Pion WebRTC peer connections via WHEP SDP exchange
//                      and receive live RTP media packets.
//   - gRPC Extractor : Clients subscribe to StreamFrames (server-streaming) and push AI metadata
//                      via PushMetadata against a local gRPC server.
//   - EventBus       : Floods system events (camera_offline, storage_warning); webhook sink
//                      exercises delivery and circuit breaker.
//   - Disk I/O       : Optional real MP4 recording to local filesystem.
//
// Usage:
//
//	go run ./cmd/loadtest \
//	  -cameras=50 -hls-viewers=50 -webrtc-viewers=20 -api-workers=50 \
//	  -grpc-pushers=10 -duration=30 -report-every=5 -output-json=bench.json -output-md=bench.md
package main

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"


	"github.com/RUSEGAL/ruseon-core/internal/api"
	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	igrpc "github.com/RUSEGAL/ruseon-core/internal/grpc"
	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	iwebrtc "github.com/RUSEGAL/ruseon-core/internal/webrtc"
	"github.com/RUSEGAL/ruseon-core/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/eventbus"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/pkg/storage/localfs"
)

// ─────────────────────────────────────────────────────────────────────────────
// CLI flags
// ─────────────────────────────────────────────────────────────────────────────

var (
	cameraCount   int
	apiWorkers    int
	hlsViewers    int
	webrtcViewers int
	grpcPushers   int
	durationSec   int
	useRealDisk   bool
	reportEvery   int
	reconnectRate int
	mode          string
	serverURL     string
	grpcURL       string
	adminPassword string
	outputJSON    string
	outputMD      string
)

func init() {
	flag.IntVar(&cameraCount, "cameras", 50, "Synthetic cameras (ingest goroutines @ 30 FPS)")
	flag.IntVar(&apiWorkers, "api-workers", 50, "Concurrent REST API workers (hits /api/*)")
	flag.IntVar(&hlsViewers, "hls-viewers", 50, "Concurrent HLS fMP4 viewers (fetches .m3u8 & segments)")
	flag.IntVar(&webrtcViewers, "webrtc-viewers", 20, "Concurrent WebRTC WHEP viewers (SDP handshake & RTP rx)")
	flag.IntVar(&grpcPushers, "grpc-pushers", 10, "Concurrent gRPC PushMetadata AI workers")
	flag.IntVar(&durationSec, "duration", 20, "Test duration in seconds")
	flag.BoolVar(&useRealDisk, "real-disk", false, "Use real localfs MP4 recording instead of /dev/null")
	flag.IntVar(&reportEvery, "report-every", 5, "Print live stats every N seconds")
	flag.IntVar(&reconnectRate, "reconnect-rate", 0, "Cameras to reconnect per second (simulates RTSP drops). 0 = disabled")
	flag.StringVar(&mode, "mode", "all", "Run mode: 'all' (in-process), 'server' (start server only), 'client' (connect to external server)")
	flag.StringVar(&serverURL, "server-url", "", "HTTP base URL of external server for client mode (e.g. http://bench-server:4197)")
	flag.StringVar(&grpcURL, "grpc-url", "", "gRPC address of external server for client mode (e.g. bench-server:4198)")
	flag.StringVar(&adminPassword, "password", "AdminPassword123!", "Admin password for client mode authentication")
	flag.StringVar(&outputJSON, "output-json", "", "Optional file path to export benchmark results as JSON")
	flag.StringVar(&outputMD, "output-md", "", "Optional file path to export benchmark results as Markdown")
	flag.Parse()
}

// ─────────────────────────────────────────────────────────────────────────────
// Latency Samplers & Atomic Counters
// ─────────────────────────────────────────────────────────────────────────────

var (
	// Samplers
	apiLatencySampler      = NewLatencySampler(20000)
	hlsPlaylistSampler     = NewLatencySampler(20000)
	hlsSegmentSampler      = NewLatencySampler(20000)
	webrtcHandshakeSampler = NewLatencySampler(5000)
	grpcStreamSampler      = NewLatencySampler(20000)

	// Ingest
	cntIngestFrames    atomic.Uint64
	cntIngestKeys      atomic.Uint64
	cntIngestBytes     atomic.Uint64
	cntDrops           atomic.Uint64
	cntReconnectsTotal atomic.Uint64

	// HTTP REST API
	cntHTTPOK        atomic.Uint64
	cntHTTPErr       atomic.Uint64
	cntHTTP2xx       atomic.Uint64
	cntHTTP4xx       atomic.Uint64
	cntHTTP5xx       atomic.Uint64
	cntHTTPBytesSent atomic.Uint64

	// HLS Delivery
	cntHLSPlaylistsOK atomic.Uint64
	cntHLSSegmentsOK  atomic.Uint64
	cntHLSErr         atomic.Uint64
	cntHLSBytesSent   atomic.Uint64

	// WebRTC WHEP
	cntWebRTCSessionsOK  atomic.Uint64
	cntWebRTCSessionsErr atomic.Uint64
	cntWebRTCRTPPackets  atomic.Uint64
	cntWebRTCBytesRx     atomic.Uint64

	// gRPC Stream & AI Meta
	cntGRPCFrames    atomic.Uint64
	cntGRPCMeta      atomic.Uint64
	cntGRPCErr       atomic.Uint64
	cntGRPCBytesSent atomic.Uint64

	// EventBus & Webhooks
	cntWebhooks  atomic.Uint64
	cntEventsPub atomic.Uint64
)

// ─────────────────────────────────────────────────────────────────────────────
// Synthetic H.264 Video Data (1280x720 30fps simulation)
// ─────────────────────────────────────────────────────────────────────────────

var (
	syntheticSPS = []byte{0x67, 0x42, 0xC0, 0x1E, 0xD9, 0x01, 0x40, 0x7B, 0x40, 0x3C, 0x22, 0x11, 0xA8}
	syntheticPPS = []byte{0x68, 0xCE, 0x38, 0x80}

	payloadIFrame = makeSyntheticNALU(0x65, 45000) // ~45 KB I-Frame
	payloadPFrame = makeSyntheticNALU(0x41, 8000)  // ~8 KB P-Frame
)

func makeSyntheticNALU(naluType byte, size int) []byte {
	b := make([]byte, size)
	b[0] = naluType
	for i := 1; i < size; i++ {
		b[i] = byte((i * 37) & 0xFF)
	}
	return b
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock Discard BlobStore for In-Memory Load Testing
// ─────────────────────────────────────────────────────────────────────────────

type discardWSC struct{ io.Writer }

func (d *discardWSC) Seek(int64, int) (int64, error) { return 0, nil }
func (d *discardWSC) Close() error                   { return nil }

type discardBlobStore struct{}

func (s *discardBlobStore) Write(_ string, _ []byte) error          { return nil }
func (s *discardBlobStore) Read(_ string) ([]byte, error)           { return nil, nil }
func (s *discardBlobStore) Delete(_ string) error                   { return nil }
func (s *discardBlobStore) Stat(_ string) (fs.FileInfo, error)      { return nil, os.ErrNotExist }
func (s *discardBlobStore) ReadDir(_ string) ([]fs.DirEntry, error) { return nil, nil }
func (s *discardBlobStore) Create(_ string) (registry.WriteSeekCloser, error) {
	return &discardWSC{io.Discard}, nil
}
func (s *discardBlobStore) Open(_ string) (io.ReadSeekCloser, error) { return nil, os.ErrNotExist }
func (s *discardBlobStore) MkdirAll(_ string) error                  { return nil }
func (s *discardBlobStore) Rename(_, _ string) error                 { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// In-Memory StateStore Mock for Ultra-Fast Benchmarking
// ─────────────────────────────────────────────────────────────────────────────

type loadtestStore struct {
	mu      sync.RWMutex
	cameras map[string]config.CameraConfig
	tags    map[string]config.TagConfig
	folders map[string]config.FolderConfig
	users   map[string]models.User
}

func newLoadtestStore() *loadtestStore {
	return &loadtestStore{
		cameras: make(map[string]config.CameraConfig),
		tags:    make(map[string]config.TagConfig),
		folders: make(map[string]config.FolderConfig),
		users:   make(map[string]models.User),
	}
}

func (s *loadtestStore) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *loadtestStore) Close() error { return nil }
func (s *loadtestStore) Sync() error  { return nil }

func (s *loadtestStore) SaveCamera(c *config.CameraConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cameras[c.ID] = *c
	return nil
}

func (s *loadtestStore) GetCamera(id string) (*config.CameraConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cameras[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &c, nil
}

func (s *loadtestStore) DeleteCamera(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cameras, id)
	return nil
}

func (s *loadtestStore) ListCameras() ([]config.CameraConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]config.CameraConfig, 0, len(s.cameras))
	for _, c := range s.cameras {
		res = append(res, c)
	}
	return res, nil
}

func (s *loadtestStore) UpdateCameraTx(id string, fn func(*config.CameraConfig) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cameras[id]
	if !ok {
		c = config.CameraConfig{ID: id}
	}
	if fn(&c) {
		s.cameras[id] = c
	}
	return nil
}

func (s *loadtestStore) BatchUpdateTraffic(updates map[string]uint64, nowMonth string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, delta := range updates {
		c, ok := s.cameras[id]
		if !ok {
			continue
		}
		if c.LastResetMonth != nowMonth {
			c.TrafficUsed = 0
			c.LastResetMonth = nowMonth
		}
		c.TrafficUsed += delta
		s.cameras[id] = c
	}
	return nil
}

func (s *loadtestStore) SaveTag(t *config.TagConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[t.ID] = *t
	return nil
}

func (s *loadtestStore) GetTag(id string) (*config.TagConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tags[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &t, nil
}

func (s *loadtestStore) DeleteTag(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tags, id)
	return nil
}

func (s *loadtestStore) ListTags() ([]config.TagConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]config.TagConfig, 0, len(s.tags))
	for _, t := range s.tags {
		res = append(res, t)
	}
	return res, nil
}

func (s *loadtestStore) SaveFolder(f *config.FolderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folders[f.ID] = *f
	return nil
}

func (s *loadtestStore) GetFolder(id string) (*config.FolderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.folders[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &f, nil
}

func (s *loadtestStore) DeleteFolder(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.folders, id)
	return nil
}

func (s *loadtestStore) ListFolders() ([]config.FolderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]config.FolderConfig, 0, len(s.folders))
	for _, f := range s.folders {
		res = append(res, f)
	}
	return res, nil
}

func (s *loadtestStore) SaveUser(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.Username] = *u
	return nil
}

func (s *loadtestStore) GetUser(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &u, nil
}

func (s *loadtestStore) ListUsers() ([]models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]models.User, 0, len(s.users))
	for _, u := range s.users {
		res = append(res, u)
	}
	return res, nil
}

func (s *loadtestStore) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, username)
	return nil
}

func (s *loadtestStore) HasUsers() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users) > 0, nil
}

func (s *loadtestStore) MigrateFromConfig(_ *config.Config) error { return nil }
func (s *loadtestStore) ExportJSON() ([]byte, error)               { return json.Marshal(s.cameras) }
func (s *loadtestStore) ImportJSON(_ []byte) error                { return nil }
func (s *loadtestStore) BackupBadger(_ io.Writer) error           { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// Layer 1: Ingest Simulation (Synthetic 30 FPS Cameras)
// ─────────────────────────────────────────────────────────────────────────────

func runSyntheticCamera(ctx context.Context, camID string, manager *stream.Manager) {
	st, ok := manager.GetStream(camID)
	if !ok {
		return
	}
	rb := st.GetRingBuffer()
	if rb == nil {
		return
	}
	rb.SetParams(nil, syntheticSPS, syntheticPPS)
	st.SetState(models.StateOnline)
	st.SetConnectedAt(time.Now())

	ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	frameIdx := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-st.Context().Done():
			return
		case <-ticker.C:
			isKey := frameIdx%30 == 0
			payload := payloadPFrame
			if isKey {
				payload = payloadIFrame
			}
			f := &buffer.Frame{
				Timestamp:  time.Duration(frameIdx) * 33 * time.Millisecond,
				IsKeyFrame: isKey,
				NALUs:      [][]byte{payload},
			}
			rb.Write(f)

			sz := uint64(len(payload))
			cntIngestFrames.Add(1)
			cntIngestBytes.Add(sz)
			st.AddBytesReceived(sz)
			st.AddFramesReceived(1)
			if isKey {
				cntIngestKeys.Add(1)
				st.AddKeyFramesReceived(1)
			}
			frameIdx++
		}
	}
}

// runReconnectSimulator симулирует периодические обрывы и переподключения камер (L5)
func runReconnectSimulator(ctx context.Context, manager *stream.Manager, camIDs []string, rate int, wg *sync.WaitGroup) {
	defer wg.Done()

	interval := time.Second / time.Duration(rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	idx := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			camID := camIDs[idx%len(camIDs)]
			idx++
			// Симулируем обрыв: удаляем и создаем заново
			manager.RemoveStream(camID)
			_ = manager.AddStream(camID, "synthetic://"+camID, useRealDisk, true /*lazyHLS*/, "tcp")
			go runSyntheticCamera(ctx, camID, manager)
			cntReconnectsTotal.Add(1)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 2: REST API Load Workers
// ─────────────────────────────────────────────────────────────────────────────

func runAPIWorker(ctx context.Context, baseURL, token string, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	endpoints := []string{
		"/api/cameras",
		"/api/stats",
		"/api/tags",
		"/api/folders",
	}

	idx := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ep := endpoints[idx%len(endpoints)]
		idx++

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+ep, nil)
		if err != nil {
			continue
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		t0 := time.Now()
		resp, err := client.Do(req)
		lat := time.Since(t0)

		if err != nil {
			if ctx.Err() == nil {
				cntHTTPErr.Add(1)
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		cntHTTPBytesSent.Add(uint64(len(body)))
		apiLatencySampler.Add(lat)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			cntHTTPOK.Add(1)
			cntHTTP2xx.Add(1)
		} else {
			cntHTTPErr.Add(1)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				cntHTTP4xx.Add(1)
			} else if resp.StatusCode >= 500 {
				cntHTTP5xx.Add(1)
			}
		}

		// Light jitter between requests (10-30ms)
		jitter := time.Duration(10+randIntn(20)) * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter):
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 3: HLS fMP4 Viewers (L3, L4: Round-robin + Jitter)
// ─────────────────────────────────────────────────────────────────────────────

func runHLSViewer(ctx context.Context, baseURL string, camIDs []string, viewerIdx int, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	camID := camIDs[viewerIdx%len(camIDs)]
	seenSegments := make(map[string]bool)

	// L4: Initial jitter (0-500ms) to prevent thundering herd
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Duration(randIntn(500)) * time.Millisecond):
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. Fetch media playlist
			t0 := time.Now()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/stream/hls/"+camID+"/stream.m3u8", nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				if ctx.Err() == nil {
					cntHLSErr.Add(1)
				}
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				cntHLSErr.Add(1)
				continue
			}

			hlsPlaylistSampler.Add(time.Since(t0))
			cntHLSPlaylistsOK.Add(1)
			cntHLSBytesSent.Add(uint64(len(body)))

			// 2. Parse segment URIs from playlist
			lines := strings.Split(string(body), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				// Download new segment
				if !seenSegments[line] {
					seenSegments[line] = true

					tSeg := time.Now()
					segURL := baseURL + "/stream/hls/" + camID + "/" + line
					segReq, segErr := http.NewRequestWithContext(ctx, http.MethodGet, segURL, nil)
					if segErr != nil {
						continue
					}
					segResp, segErr := client.Do(segReq)
					if segErr != nil {
						if ctx.Err() == nil {
							cntHLSErr.Add(1)
						}
						continue
					}

					segBody, _ := io.ReadAll(segResp.Body)
					_ = segResp.Body.Close()

					if segResp.StatusCode == http.StatusOK {
						hlsSegmentSampler.Add(time.Since(tSeg))
						cntHLSSegmentsOK.Add(1)
						cntHLSBytesSent.Add(uint64(len(segBody)))
					} else {
						cntHLSErr.Add(1)
					}
				}
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 4: WebRTC (WHEP) Viewers
// ─────────────────────────────────────────────────────────────────────────────

func runWebRTCViewer(ctx context.Context, baseURL string, camIDs []string, engine *iwebrtc.Engine, wg *sync.WaitGroup) {
	defer wg.Done()

	camID := camIDs[randIntn(len(camIDs))]

	if engine == nil {
		return
	}

	pc, err := engine.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}
	defer pc.Close()

	_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		buf := make([]byte, 1600)
		for {
			if ctx.Err() != nil {
				return
			}
			n, _, rtpErr := track.Read(buf)
			if rtpErr != nil {
				return
			}
			cntWebRTCRTPPackets.Add(1)
			// #nosec G115 -- packet size is non-negative
			cntWebRTCBytesRx.Add(uint64(n))
		}
	})

	t0 := time.Now()
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	select {
	case <-gatherComplete:
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return
	}

	localDesc := pc.LocalDescription()
	if localDesc == nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/stream/webrtc/whep/"+camID, strings.NewReader(localDesc.SDP))
	if err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}
	req.Header.Set("Content-Type", "application/sdp")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	answerSDP, err := io.ReadAll(resp.Body)
	if err != nil || len(answerSDP) == 0 {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  string(answerSDP),
	}); err != nil {
		cntWebRTCSessionsErr.Add(1)
		return
	}

	webrtcHandshakeSampler.Add(time.Since(t0))
	cntWebRTCSessionsOK.Add(1)

	// Keep session alive for the test duration
	<-ctx.Done()
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 5: gRPC StreamFrames Consumer
// ─────────────────────────────────────────────────────────────────────────────

func runGRPCFrameConsumer(ctx context.Context, grpcAddr, camID string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cntGRPCErr.Add(1)
		return
	}
	defer conn.Close()

	streamClient, err := pb.NewFrameServiceClient(conn).StreamFrames(ctx, &pb.StreamRequest{CameraId: camID})
	if err != nil {
		cntGRPCErr.Add(1)
		return
	}

	for {
		t0 := time.Now()
		resp, err := streamClient.Recv()
		if err != nil {
			if ctx.Err() == nil {
				cntGRPCErr.Add(1)
			}
			return
		}
		grpcStreamSampler.Add(time.Since(t0))
		cntGRPCFrames.Add(1)
		if resp != nil {
			cntGRPCBytesSent.Add(uint64(len(resp.Payload)))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 6: gRPC Metadata Pusher (AI Ingestion Simulation)
// ─────────────────────────────────────────────────────────────────────────────

func runGRPCMetaPusher(ctx context.Context, grpcAddr string, camIDs []string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cntGRPCErr.Add(1)
		return
	}
	defer conn.Close()

	pushStream, err := pb.NewFrameServiceClient(conn).PushMetadata(ctx)
	if err != nil {
		cntGRPCErr.Add(1)
		return
	}

	ticker := time.NewTicker(10 * time.Millisecond) // 100 metadata pkts/sec
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_, _ = pushStream.CloseAndRecv()
			return
		case <-ticker.C:
			camID := camIDs[randIntn(len(camIDs))]
			err := pushStream.Send(&pb.MetadataRequest{
				CameraId: camID,
				Objects: []*pb.BoundingBox{
					{X: 0.10, Y: 0.10, Width: 0.30, Height: 0.50, Label: "person", Confidence: 0.92},
					{X: 0.60, Y: 0.20, Width: 0.20, Height: 0.40, Label: "vehicle", Confidence: 0.87},
				},
			})
			if err != nil {
				cntGRPCErr.Add(1)
				return
			}
			cntGRPCMeta.Add(1)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 7: EventBus Flood & Webhook Sink
// ─────────────────────────────────────────────────────────────────────────────

func startWebhookSink() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		cntWebhooks.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
}

func runEventBusFlood(ctx context.Context, camIDs []string, wg *sync.WaitGroup) {
	defer wg.Done()

	topics := []string{"camera_offline", "camera_online", "recording_failed", "storage_warning"}
	ticker := time.NewTicker(5 * time.Millisecond) // 200 events/sec per goroutine
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if registry.CurrentEventBus == nil {
				continue
			}
			topic := topics[randIntn(len(topics))]
			camID := camIDs[randIntn(len(camIDs))]
			registry.CurrentEventBus.Publish(topic, camID, map[string]string{"source": "loadtest"})
			cntEventsPub.Add(1)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Live Stats & Terminal Display
// ─────────────────────────────────────────────────────────────────────────────

func printLive(elapsed time.Duration) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fi := cntIngestFrames.Load()
	hOK := cntHTTPOK.Load()
	hErr := cntHTTPErr.Load()
	hlsSegs := cntHLSSegmentsOK.Load()
	webrtcPkts := cntWebRTCRTPPackets.Load()
	gF := cntGRPCFrames.Load()
	gM := cntGRPCMeta.Load()
	gE := cntGRPCErr.Load()
	wh := cntWebhooks.Load()

	apiStats := apiLatencySampler.Calculate()

	reconnStr := ""
	if reconnectRate > 0 {
		reconnStr = fmt.Sprintf(" reconn=%d |", cntReconnectsTotal.Load())
	}

	fmt.Printf(
		"[%4.0fs] Ingest=%d (fps=%.0f, drops=%d) |%s API_OK=%d Err=%d (p50=%.1fms, p95=%.1fms) | HLS_Segs=%d | WebRTC_Pkts=%d | gRPC_FPS=%.0f (meta=%d, err=%d) | Webhooks=%d | Heap=%dMB | Goroutines=%d\n",
		elapsed.Seconds(),
		fi, float64(fi)/elapsed.Seconds(), cntDrops.Load(),
		reconnStr,
		hOK, hErr, apiStats.P50Ms, apiStats.P95Ms,
		hlsSegs,
		webrtcPkts,
		float64(gF)/elapsed.Seconds(), gM, gE,
		wh,
		m.HeapAlloc/1024/1024,
		runtime.NumGoroutine(),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// main & Submodes (L8)
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	switch mode {
	case "server":
		runServerMode()
	case "client":
		if serverURL == "" || grpcURL == "" {
			fmt.Fprintln(os.Stderr, "[fatal] -server-url and -grpc-url required in client mode")
			os.Exit(1)
		}
		runClientMode(serverURL, grpcURL)
	default:
		runAllInProcess()
	}
}

func runServerMode() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	fmt.Printf("[server] Starting in server mode (cameras=%d, real-disk=%v)\n", cameraCount, useRealDisk)

	cfg := &config.Config{}
	cfg.Auth.Secret = "loadtest-secret-32chars-minimum!!"

	store := newLoadtestStore()
	registry.RegisterStateStore(store)

	if useRealDisk {
		recDir := "./recordings"
		_ = os.MkdirAll(recDir, 0750)
		registry.RegisterBlobStore(localfs.NewLocalFS(recDir))
	} else {
		registry.RegisterBlobStore(&discardBlobStore{})
	}

	// Create admin user for client authentication
	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	_ = store.SaveUser(&models.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
	})

	authenticator := auth.NewLocalAuthenticator(cfg)
	registry.RegisterAuthenticator(authenticator)



	manager := stream.NewManager()
	camIDs := make([]string, cameraCount)
	for i := 0; i < cameraCount; i++ {
		camIDs[i] = fmt.Sprintf("cam_%03d", i)
		_ = store.SaveCamera(&config.CameraConfig{
			ID:          camIDs[i],
			URL:         "synthetic://" + camIDs[i],
			Record:      useRealDisk,
			TrafficUsed: 0,
		})
		_ = manager.AddStream(camIDs[i], "synthetic://"+camIDs[i], useRealDisk, true, "tcp")
	}

	webrtcEngine, _ := iwebrtc.NewEngine(cfg)
	handler := api.NewHandler(manager, cfg, store, webrtcEngine)
	router := api.SetupRouter(handler, authenticator, false, nil)

	httpPort := 4197
	grpcPort := 4198

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.ListenAndServe() }()

	grpcSrv := igrpc.NewServer(manager, nil, "", "")
	go func() { _ = grpcSrv.Start(fmt.Sprintf(":%d", grpcPort)) }()

	fmt.Printf("[server] HTTP: http://0.0.0.0:%d\n", httpPort)
	fmt.Printf("[server] gRPC: 0.0.0.0:%d\n", grpcPort)
	fmt.Printf("[server] Admin credentials: admin / %s\n", adminPassword)
	fmt.Println("[server] Ingesting synthetic camera streams...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, id := range camIDs {
		go runSyntheticCamera(ctx, id, manager)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("[server] Ready. Waiting for SIGINT/SIGTERM...")
	<-quit

	fmt.Println("[server] Shutting down...")
	cancel()
	manager.Close()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	_ = srv.Close()
	grpcSrv.Stop()
	if webrtcEngine != nil {
		_ = webrtcEngine.Close()
	}
	fmt.Println("[server] Shutdown complete.")
}


func authenticateClient(baseURL, username, password string) (string, error) {
	loginPayload, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(baseURL+"/api/login", "application/json", bytes.NewReader(loginPayload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed with status %d", resp.StatusCode)
	}

	var res struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Token, nil
}

func fetchCamIDs(baseURL, token string, fallbackCount int) []string {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/cameras", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		var cams []struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(resp.Body).Decode(&cams) == nil && len(cams) > 0 {
			_ = resp.Body.Close()
			ids := make([]string, len(cams))
			for i, c := range cams {
				ids[i] = c.ID
			}
			return ids
		}
		_ = resp.Body.Close()
	}

	ids := make([]string, fallbackCount)
	for i := 0; i < fallbackCount; i++ {
		ids[i] = fmt.Sprintf("cam_%03d", i)
	}
	return ids
}

func runClientMode(targetServerURL, targetGRPCURL string) {
	log.Logger = zerolog.New(io.Discard)

	banner()
	fmt.Println("[client] Connecting to external server:", targetServerURL)

	token, err := authenticateClient(targetServerURL, "admin", adminPassword)
	if err != nil {
		fmt.Printf("[client] [warn] Login failed (%v), proceeding with fallback JWT...\n", err)
		token = mustMakeJWT("loadtest-secret-32chars-minimum!!")
	} else {
		fmt.Println("[client] Authenticated successfully with server.")
	}

	camIDs := fetchCamIDs(targetServerURL, token, cameraCount)
	fmt.Printf("[client] Target cameras: %d | API workers: %d | HLS: %d | WebRTC: %d\n",
		len(camIDs), apiWorkers, hlsViewers, webrtcViewers)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(durationSec)*time.Second)
	defer cancel()

	hw := newHWCollector()
	go hw.runSampler(ctx)

	clientWebRTCEngine, _ := iwebrtc.NewEngine(nil)
	if clientWebRTCEngine != nil {
		defer clientWebRTCEngine.Close()
	}

	var wg sync.WaitGroup
	start := time.Now()
	fmt.Println("[client] Running load test...")

	// Layer 2: API Workers
	for i := 0; i < apiWorkers; i++ {
		wg.Add(1)
		go runAPIWorker(ctx, targetServerURL, token, &wg)
	}

	// Layer 3: HLS Viewers
	for i := 0; i < hlsViewers; i++ {
		wg.Add(1)
		go runHLSViewer(ctx, targetServerURL, camIDs, i, &wg)
	}

	// Layer 4: WebRTC Viewers
	for i := 0; i < webrtcViewers; i++ {
		wg.Add(1)
		go runWebRTCViewer(ctx, targetServerURL, camIDs, clientWebRTCEngine, &wg)
	}

	// Layer 5: gRPC Consumers
	for _, id := range camIDs {
		wg.Add(1)
		go runGRPCFrameConsumer(ctx, targetGRPCURL, id, &wg)
	}

	// Layer 6: gRPC Meta Pushers
	for i := 0; i < grpcPushers; i++ {
		wg.Add(1)
		go runGRPCMetaPusher(ctx, targetGRPCURL, camIDs, &wg)
	}

	statsTick := time.NewTicker(time.Duration(reportEvery) * time.Second)
	defer statsTick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsTick.C:
				printLive(time.Since(start))
			}
		}
	}()

	wg.Wait()
	elapsed := time.Since(start)
	sec := elapsed.Seconds()

	cpuAvg, cpuPeak, procCPU, rssMB, rxMbps, txMbps := hw.result()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	result := BenchmarkResult{
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		DurationSec: math.Round(sec*100) / 100,
		Config: BenchmarkConfig{
			Cameras:       len(camIDs),
			APIWorkers:    apiWorkers,
			HLSViewers:    hlsViewers,
			WebRTCViewers: webrtcViewers,
			GRPCPushers:   grpcPushers,
			DurationSec:   durationSec,
			Mode:          "client",
		},
		HTTP: HTTPMetrics{
			TotalOK:   cntHTTPOK.Load(),
			TotalErr:  cntHTTPErr.Load(),
			RPS:       math.Round(float64(cntHTTPOK.Load())/sec*10) / 10,
			BytesSent: cntHTTPBytesSent.Load(),
			Latency:   apiLatencySampler.Calculate(),
			Status2xx: cntHTTP2xx.Load(),
			Status4xx: cntHTTP4xx.Load(),
			Status5xx: cntHTTP5xx.Load(),
		},
		HLS: HLSMetrics{
			PlaylistsOK:   cntHLSPlaylistsOK.Load(),
			SegmentsOK:    cntHLSSegmentsOK.Load(),
			Errors:        cntHLSErr.Load(),
			BytesSent:     cntHLSBytesSent.Load(),
			ThroughputMBs: math.Round(float64(cntHLSBytesSent.Load())/sec/1024/1024*100) / 100,
			PlaylistLat:   hlsPlaylistSampler.Calculate(),
			SegmentLat:    hlsSegmentSampler.Calculate(),
		},
		WebRTC: WebRTCMetrics{
			SessionsOK:    cntWebRTCSessionsOK.Load(),
			SessionsErr:   cntWebRTCSessionsErr.Load(),
			RTPPacketsRx:  cntWebRTCRTPPackets.Load(),
			BytesRx:       cntWebRTCBytesRx.Load(),
			ThroughputMBs: math.Round(float64(cntWebRTCBytesRx.Load())/sec/1024/1024*100) / 100,
			HandshakeLat:  webrtcHandshakeSampler.Calculate(),
		},
		GRPC: GRPCMetrics{
			FramesSent: cntGRPCFrames.Load(),
			MetaPushed: cntGRPCMeta.Load(),
			Errors:     cntGRPCErr.Load(),
			FPS:        math.Round(float64(cntGRPCFrames.Load())/sec*10) / 10,
			MetaRPS:    math.Round(float64(cntGRPCMeta.Load())/sec*10) / 10,
			BytesSent:  cntGRPCBytesSent.Load(),
			StreamLat:  grpcStreamSampler.Calculate(),
		},
		System: SystemMetrics{
			NumCPU:        runtime.NumCPU(),
			GoroutinesEnd: runtime.NumGoroutine(),
			HeapAllocMB:   m.HeapAlloc / 1024 / 1024,
			HeapInuseMB:   m.HeapInuse / 1024 / 1024,
			HeapSysMB:     m.HeapSys / 1024 / 1024,
			SysMB:         m.Sys / 1024 / 1024,
			NumGC:         m.NumGC,
			GCPauseTotal:  math.Round(float64(m.PauseTotalNs)/1e6*100) / 100,
			GCPauseMax:    gcMaxPauseMs(&m),
			CPUPctAvg:     math.Round(cpuAvg*10) / 10,
			CPUPctPeak:    math.Round(cpuPeak*10) / 10,
			ProcCPUPct:    math.Round(procCPU*10) / 10,
			RSSMB:         rssMB,
			NetRxMbps:     math.Round(rxMbps*10) / 10,

			NetTxMbps:     math.Round(txMbps*10) / 10,
		},
	}

	printFinalDashboard(&result)
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════════════════════╝")

	if outputJSON != "" {
		_ = result.ExportJSON(outputJSON)
		fmt.Printf("[export] JSON saved to: %s\n", outputJSON)
	}
	if outputMD != "" {
		_ = result.ExportMarkdown(outputMD)
		fmt.Printf("[export] Markdown saved to: %s\n", outputMD)
	}
}

func runAllInProcess() {
	// Suppress internal zerolog so it doesn't pollute benchmark stats
	log.Logger = zerolog.New(io.Discard)

	banner()

	// ── 1. Storage & Temp Directories ────────────────────────────────────

	tmpRecDir := filepath.Join(os.TempDir(), fmt.Sprintf("ruseon_loadtest_%d", time.Now().UnixNano()))
	if useRealDisk {
		_ = os.MkdirAll(tmpRecDir, 0750)
		registry.RegisterBlobStore(localfs.NewLocalFS(tmpRecDir))
		fmt.Printf("[setup] BlobStore : real disk (%s)\n", tmpRecDir)
		defer os.RemoveAll(tmpRecDir)
	} else {
		registry.RegisterBlobStore(&discardBlobStore{})
		fmt.Println("[setup] BlobStore : /dev/null (In-Memory)")
	}

	// ── 2. In-Memory StateStore ──────────────────────────────────────────

	store := newLoadtestStore()
	registry.RegisterStateStore(store)

	// Pre-populate cameras in store
	camIDs := make([]string, cameraCount)
	for i := range camIDs {
		id := fmt.Sprintf("lt_%03d", i)
		camIDs[i] = id
		_ = store.SaveCamera(&config.CameraConfig{
			ID:            id,
			URL:           "synthetic://" + id,
			Record:        useRealDisk,
			RetentionDays: 7,
			Tags:          []string{"loadtest", "synthetic"},
			FolderID:      "root",
			LazyHLS:       true,
			Transport:     "tcp",
		})
	}
	_ = store.SaveTag(&config.TagConfig{ID: "loadtest", Name: "Load Test Cameras"})
	_ = store.SaveFolder(&config.FolderConfig{ID: "root", Name: "Default Folder"})

	// ── 3. Config & Auth ─────────────────────────────────────────────────

	cfg := &config.Config{}
	cfg.Auth.Secret = "loadtest-secret-32chars-minimum!!"
	authenticator := auth.NewLocalAuthenticator(cfg)
	registry.RegisterAuthenticator(authenticator)
	_ = store.SaveUser(&models.User{
		Username: "loadtest-admin",
		Role:     models.RoleAdmin,
	})
	adminToken := mustMakeJWT(cfg.Auth.Secret)


	// ── 4. EventBus + Webhook Sink ───────────────────────────────────────

	sink := startWebhookSink()
	defer sink.Close()

	bus := eventbus.New(config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{URL: sink.URL},
		},
	}, 4)
	registry.RegisterEventBus(bus)
	defer bus.Stop()

	// ── 5. Stream Manager & Ingest Setup ──────────────────────────────────

	manager := stream.NewManager()
	for _, id := range camIDs {
		_ = manager.AddStream(id, "synthetic://"+id, useRealDisk, true /*lazyHLS*/, "tcp")
	}

	// ── 6. WebRTC Engine & HTTP API Server (Gin) ─────────────────────────

	webrtcEngine, _ := iwebrtc.NewEngine(cfg)
	if webrtcEngine != nil {
		defer webrtcEngine.Close()
	}

	clientWebRTCEngine, _ := iwebrtc.NewEngine(nil)
	if clientWebRTCEngine != nil {
		defer clientWebRTCEngine.Close()
	}

	handler := api.NewHandler(manager, cfg, store, webrtcEngine)
	router := api.SetupRouter(handler, authenticator, false /*debug*/, nil)
	httpSrv := httptest.NewServer(router)
	defer httpSrv.Close()

	// ── 7. gRPC Server ────────────────────────────────────────────────────

	grpcSrv := igrpc.NewServer(manager, nil, "", "")
	grpcAddr, err := freeAddr()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fatal] gRPC port: %v\n", err)
		return
	}
	go func() { _ = grpcSrv.Start(grpcAddr) }()
	time.Sleep(500 * time.Millisecond) // let gRPC bind
	defer grpcSrv.Stop()

	fmt.Printf("[setup] HTTP Server: %s\n", httpSrv.URL)
	fmt.Printf("[setup] gRPC Server: %s\n", grpcAddr)
	fmt.Printf("[setup] Webhook Sink: %s\n", sink.URL)
	fmt.Printf("[setup] Cameras: %d | API Workers: %d | HLS Viewers: %d | WebRTC: %d\n\n",
		cameraCount, apiWorkers, hlsViewers, webrtcViewers)

	// ── 8. Execution Context & Workers ────────────────────────────────────

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(durationSec)*time.Second)
	defer cancel()

	hw := newHWCollector()
	go hw.runSampler(ctx)

	var wg sync.WaitGroup
	start := time.Now()
	fmt.Println("[run] Starting full-stack load test...")

	// Layer 1: Ingest (one synthetic camera per cameraID)
	for _, id := range camIDs {
		go runSyntheticCamera(ctx, id, manager)
	}

	// L5: Reconnect simulator (if enabled)
	if reconnectRate > 0 {
		wg.Add(1)
		go runReconnectSimulator(ctx, manager, camIDs, reconnectRate, &wg)
	}

	// L1: Live drops collector loop
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				var d uint64
				for _, st := range manager.GetStreams() {
					if rb := st.GetRingBuffer(); rb != nil {
						d += rb.GetTotalDrops()
					}
				}
				cntDrops.Store(d)
			}
		}
	}()

	// Warm-up: wait for first keyframes to populate RingBuffers
	time.Sleep(400 * time.Millisecond)

	// Layer 2: REST API Workers
	for i := 0; i < apiWorkers; i++ {
		wg.Add(1)
		go runAPIWorker(ctx, httpSrv.URL, adminToken, &wg)
	}

	// Layer 3: HLS fMP4 Viewers (L3, L4: round-robin + jitter)
	for i := 0; i < hlsViewers; i++ {
		wg.Add(1)
		go runHLSViewer(ctx, httpSrv.URL, camIDs, i, &wg)
	}

	// Layer 4: WebRTC (WHEP) Viewers
	for i := 0; i < webrtcViewers; i++ {
		wg.Add(1)
		go runWebRTCViewer(ctx, httpSrv.URL, camIDs, clientWebRTCEngine, &wg)
	}

	// Layer 5: gRPC StreamFrames Consumers
	for _, id := range camIDs {
		wg.Add(1)
		go runGRPCFrameConsumer(ctx, grpcAddr, id, &wg)
	}

	// Layer 6: gRPC Metadata Pushers
	for i := 0; i < grpcPushers; i++ {
		wg.Add(1)
		go runGRPCMetaPusher(ctx, grpcAddr, camIDs, &wg)
	}

	// Layer 7: EventBus Flood
	wg.Add(2)
	go runEventBusFlood(ctx, camIDs, &wg)
	go runEventBusFlood(ctx, camIDs, &wg)

	// Live telemetry ticker
	statsTick := time.NewTicker(time.Duration(reportEvery) * time.Second)
	defer statsTick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsTick.C:
				printLive(time.Since(start))
			}
		}
	}()

	// ── 9. Wait for Completion ────────────────────────────────────────────

	wg.Wait()
	elapsed := time.Since(start)
	sec := elapsed.Seconds()

	activeGoroutines := runtime.NumGoroutine()

	// ── 10. Graceful Teardown & Post-Teardown Verification ──────────────
	manager.Close()
	httpSrv.Close()
	grpcSrv.Stop()
	if webrtcEngine != nil {
		// #nosec G104 -- best-effort shutdown during benchmark teardown
		_ = webrtcEngine.Close()
	}
	if clientWebRTCEngine != nil {
		// #nosec G104 -- best-effort shutdown during benchmark teardown
		_ = clientWebRTCEngine.Close()
	}
	bus.Stop()
	sink.Close()

	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cpuAvg, cpuPeak, procCPU, rssMB, rxMbps, txMbps := hw.result()

	var totalDrops uint64
	for _, st := range manager.GetStreams() {
		if rb := st.GetRingBuffer(); rb != nil {
			totalDrops += rb.GetTotalDrops()
		}
	}
	if totalDrops == 0 {
		totalDrops = cntDrops.Load()
	}

	// Disk storage stats
	var diskBytesWritten uint64
	var diskFilesCreated int
	if useRealDisk {
		_ = filepath.Walk(tmpRecDir, func(_ string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() {
				diskFilesCreated++
				// #nosec G115 -- file size is non-negative
				diskBytesWritten += uint64(info.Size())
			}
			return nil
		})
	}

	// Compile Latencies
	apiLat := apiLatencySampler.Calculate()
	hlsPlayLat := hlsPlaylistSampler.Calculate()
	hlsSegLat := hlsSegmentSampler.Calculate()
	webrtcLat := webrtcHandshakeSampler.Calculate()
	grpcLat := grpcStreamSampler.Calculate()

	// Calculate Bitrates & Throughputs
	ingestFrames := cntIngestFrames.Load()
	ingestBytes := cntIngestBytes.Load()
	ingestMbps := float64(ingestBytes*8) / sec / 1e6

	hOK := cntHTTPOK.Load()
	hErr := cntHTTPErr.Load()
	apiRPS := float64(hOK) / sec

	hlsSegs := cntHLSSegmentsOK.Load()
	hlsBytes := cntHLSBytesSent.Load()
	hlsMBs := float64(hlsBytes) / sec / 1024 / 1024

	webrtcPkts := cntWebRTCRTPPackets.Load()
	webrtcBytes := cntWebRTCBytesRx.Load()
	webrtcMBs := float64(webrtcBytes) / sec / 1024 / 1024

	gF := cntGRPCFrames.Load()
	gM := cntGRPCMeta.Load()
	gE := cntGRPCErr.Load()

	wh := cntWebhooks.Load()
	evPub := cntEventsPub.Load()

	droppedEvents := uint64(0)
	if evPub > wh {
		droppedEvents = evPub - wh
	}
	dropPct := 0.0
	if evPub > 0 {
		dropPct = math.Round(float64(droppedEvents)/float64(evPub)*1000) / 10
	}

	// ── 11. Compile BenchmarkResult ───────────────────────────────────────

	result := BenchmarkResult{
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		DurationSec: math.Round(sec*100) / 100,
		Config: BenchmarkConfig{
			Cameras:       cameraCount,
			APIWorkers:    apiWorkers,
			HLSViewers:    hlsViewers,
			WebRTCViewers: webrtcViewers,
			GRPCPushers:   grpcPushers,
			DurationSec:   durationSec,
			RealDisk:      useRealDisk,
			ReconnectRate: reconnectRate,
			Mode:          "all",
		},
		Ingest: IngestMetrics{
			TotalFrames: ingestFrames,
			KeyFrames:   cntIngestKeys.Load(),
			TotalBytes:  ingestBytes,
			FPS:         math.Round(float64(ingestFrames)/sec*10) / 10,
			BitrateMbps: math.Round(ingestMbps*10) / 10,
			Drops:       totalDrops,
			Reconnects:  cntReconnectsTotal.Load(),
		},
		HTTP: HTTPMetrics{
			TotalOK:   hOK,
			TotalErr:  hErr,
			RPS:       math.Round(apiRPS*10) / 10,
			BytesSent: cntHTTPBytesSent.Load(),
			Latency:   apiLat,
			Status2xx: cntHTTP2xx.Load(),
			Status4xx: cntHTTP4xx.Load(),
			Status5xx: cntHTTP5xx.Load(),
		},
		HLS: HLSMetrics{
			PlaylistsOK:   cntHLSPlaylistsOK.Load(),
			SegmentsOK:    hlsSegs,
			Errors:        cntHLSErr.Load(),
			BytesSent:     hlsBytes,
			ThroughputMBs: math.Round(hlsMBs*100) / 100,
			PlaylistLat:   hlsPlayLat,
			SegmentLat:    hlsSegLat,
		},
		WebRTC: WebRTCMetrics{
			SessionsOK:    cntWebRTCSessionsOK.Load(),
			SessionsErr:   cntWebRTCSessionsErr.Load(),
			RTPPacketsRx:  webrtcPkts,
			BytesRx:       webrtcBytes,
			ThroughputMBs: math.Round(webrtcMBs*100) / 100,
			HandshakeLat:  webrtcLat,
		},
		GRPC: GRPCMetrics{
			FramesSent: gF,
			MetaPushed: gM,
			Errors:     gE,
			FPS:        math.Round(float64(gF)/sec*10) / 10,
			MetaRPS:    math.Round(float64(gM)/sec*10) / 10,
			BytesSent:  cntGRPCBytesSent.Load(),
			StreamLat:  grpcLat,
		},
		EventBus: EventBusMetrics{
			Published:  evPub,
			Delivered:  wh,
			Dropped:    droppedEvents,
			DropPct:    dropPct,
			RatePerSec: math.Round(float64(wh)/sec*10) / 10,
		},
		Storage: StorageMetrics{
			RealDisk:     useRealDisk,
			BytesWritten: diskBytesWritten,
			WriteMBs:     math.Round(float64(diskBytesWritten)/sec/1024/1024*100) / 100,
			FilesCreated: diskFilesCreated,
		},
		System: SystemMetrics{
			NumCPU:        runtime.NumCPU(),
			GoroutinesEnd: activeGoroutines,
			HeapAllocMB:   m.HeapAlloc / 1024 / 1024,
			HeapInuseMB:   m.HeapInuse / 1024 / 1024,
			HeapSysMB:     m.HeapSys / 1024 / 1024,
			TotalAllocMB:  m.TotalAlloc / 1024 / 1024,
			SysMB:         m.Sys / 1024 / 1024,
			NumGC:         m.NumGC,
			GCPauseTotal:  math.Round(float64(m.PauseTotalNs)/1e6*100) / 100,
			GCPauseMax:    gcMaxPauseMs(&m),
			CPUPctAvg:     math.Round(cpuAvg*10) / 10,
			CPUPctPeak:    math.Round(cpuPeak*10) / 10,
			ProcCPUPct:    math.Round(procCPU*10) / 10,
			RSSMB:         rssMB,
			NetRxMbps:     math.Round(rxMbps*10) / 10,
			NetTxMbps:     math.Round(txMbps*10) / 10,
		},
	}

	// ── 12. Print Final Dashboard ─────────────────────────────────────────

	printFinalDashboard(&result)
	fmt.Printf("║  [Teardown Check] Baseline Goroutines Post-Cleanup: %-4d                                      ║\n", baselineGoroutines)
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════════════════════╝")

	// ── 13. Export JSON / Markdown ────────────────────────────────────────

	if outputJSON != "" {
		if err := result.ExportJSON(outputJSON); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] Failed to export JSON: %v\n", err)
		} else {
			fmt.Printf("[export] JSON saved to: %s\n", outputJSON)
		}
	}

	if outputMD != "" {
		if err := result.ExportMarkdown(outputMD); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] Failed to export Markdown: %v\n", err)
		} else {
			fmt.Printf("[export] Markdown saved to: %s\n", outputMD)
		}
	}

	if hOK > 0 && hErr > hOK/10 {
		fmt.Fprintln(os.Stderr, "\n[WARN] High error rate detected — check logs/routing")
		return
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Visual Dashboard & Helpers
// ─────────────────────────────────────────────────────────────────────────────

func gcMaxPauseMs(m *runtime.MemStats) float64 {
	n := int(m.NumGC)
	if n > 256 {
		n = 256
	}
	var maxNs uint64
	for i := 0; i < n; i++ {
		if m.PauseNs[i] > maxNs {
			maxNs = m.PauseNs[i]
		}
	}
	return math.Round(float64(maxNs)/1e6*100) / 100
}

func printFinalDashboard(r *BenchmarkResult) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                  FINAL BENCHMARK REPORT                                      ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  ⚠  In-process: API/HLS latency excludes network RTT. WebRTC uses real Pion P2P.            ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Elapsed: %-8.1fs   CPU Cores: %-4d   Goroutines: %-5d   Heap: %-4dMB   Sys: %-5dMB     ║\n",
		r.DurationSec, r.System.NumCPU, r.System.GoroutinesEnd, r.System.HeapAllocMB, r.System.SysMB)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════════════════╣")
	reconnStr := ""
	if r.Ingest.Reconnects > 0 {
		reconnStr = fmt.Sprintf("  Reconn: %-4d", r.Ingest.Reconnects)
	}
	fmt.Printf("║  [Ingest (30 FPS)]  Frames: %-8d  Keyframes: %-7d  FPS: %-6.0f  Bandwidth: %-5.1f Mbps%s ║\n",
		r.Ingest.TotalFrames, r.Ingest.KeyFrames, r.Ingest.FPS, r.Ingest.BitrateMbps, reconnStr)
	fmt.Printf("║  [Drops Tracked]    Total Drops: %-8d                                                        ║\n",
		r.Ingest.Drops)
	fmt.Printf("║  [REST API]         Requests: %-6d (OK: %-5d, Err: %-3d)   Throughput: %-5.0f RPS            ║\n",
		r.HTTP.TotalOK+r.HTTP.TotalErr, r.HTTP.TotalOK, r.HTTP.TotalErr, r.HTTP.RPS)
	fmt.Printf("║                     Latency:  p50=%.2fms  p90=%.2fms  p95=%.2fms  p99=%.2fms  max=%.2fms    ║\n",
		r.HTTP.Latency.P50Ms, r.HTTP.Latency.P90Ms, r.HTTP.Latency.P95Ms, r.HTTP.Latency.P99Ms, r.HTTP.Latency.MaxMs)
	fmt.Printf("║  [HLS fMP4]         Playlists: %-5d   Segments: %-6d   Throughput: %-5.2f MB/s              ║\n",
		r.HLS.PlaylistsOK, r.HLS.SegmentsOK, r.HLS.ThroughputMBs)
	fmt.Printf("║                     Segment Latency: p50=%.2fms  p95=%.2fms  p99=%.2fms (err: %d)          ║\n",
		r.HLS.SegmentLat.P50Ms, r.HLS.SegmentLat.P95Ms, r.HLS.SegmentLat.P99Ms, r.HLS.Errors)
	fmt.Printf("║  [WebRTC WHEP]      Sessions OK: %-4d (err: %-2d)   RTP Packets: %-7d   Egress: %-5.2f MB/s  ║\n",
		r.WebRTC.SessionsOK, r.WebRTC.SessionsErr, r.WebRTC.RTPPacketsRx, r.WebRTC.ThroughputMBs)
	fmt.Printf("║                     Handshake Latency: p50=%.2fms  p95=%.2fms  max=%.2fms                 ║\n",
		r.WebRTC.HandshakeLat.P50Ms, r.WebRTC.HandshakeLat.P95Ms, r.WebRTC.HandshakeLat.MaxMs)
	fmt.Printf("║  [gRPC Frames & AI] Stream FPS: %-6.0f (err: %-2d)  Meta RPS: %-6.0f   Delivery: p50=%.2fms  ║\n",
		r.GRPC.FPS, r.GRPC.Errors, r.GRPC.MetaRPS, r.GRPC.StreamLat.P50Ms)
	fmt.Printf("║  [EventBus]         Published: %-6d  Delivered: %-6d  Dropped: %-5d (%.1f%%)             ║\n",
		r.EventBus.Published, r.EventBus.Delivered, r.EventBus.Dropped, r.EventBus.DropPct)
	if r.Storage.RealDisk {
		fmt.Printf("║  [Disk I/O Storage] Files: %-5d   Write Rate: %-5.2f MB/s                                   ║\n",
			r.Storage.FilesCreated, r.Storage.WriteMBs)
	}
	fmt.Printf("║  [GC Telemetry]     Cycles: %-5d   Pause Total: %-6.2f ms   Max Pause: %-5.2f ms             ║\n",
		r.System.NumGC, r.System.GCPauseTotal, r.System.GCPauseMax)
	if r.System.CPUPctAvg > 0 || r.System.CPUPctPeak > 0 || r.System.RSSMB > 0 {
		fmt.Printf("║  [CPU / Memory]     CPU avg=%.1f%%  peak=%.1f%%  proc=%.1f%%   RSS: %d MB                     ║\n",
			r.System.CPUPctAvg, r.System.CPUPctPeak, r.System.ProcCPUPct, r.System.RSSMB)
	}
	if r.System.NetRxMbps > 0 || r.System.NetTxMbps > 0 {
		fmt.Printf("║  [Network (OS)]     RX=%.1f Mbps   TX=%.1f Mbps                                            ║\n",
			r.System.NetRxMbps, r.System.NetTxMbps)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func mustMakeJWT(secret string) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": "loadtest-admin",
		"role":     string(models.RoleAdmin),
		"exp":      time.Now().Add(2 * time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		panic("loadtest: failed to sign JWT: " + err.Error())
	}
	return s
}

func freeAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr, nil
}

func randIntn(maxVal int) int {
	if maxVal <= 0 {
		return 0
	}
	n, err := crand.Int(crand.Reader, big.NewInt(int64(maxVal)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

func banner() {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║              RUSEON Core — Full-Stack Load Test v2.0                  ║")
	fmt.Println("  ╠═══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  ║  cameras=%-4d  api-workers=%-4d  hls-viewers=%-4d  webrtc-viewers=%-3d  ║\n",
		cameraCount, apiWorkers, hlsViewers, webrtcViewers)
	fmt.Printf("  ║  duration=%-3ds  real-disk=%-5v    report-every=%-2ds   grpc-pushers=%-3d   ║\n",
		durationSec, useRealDisk, reportEvery, grpcPushers)
	if reconnectRate > 0 {
		fmt.Printf("  ║  reconnect-rate=%-3d/sec                                              ║\n", reconnectRate)
	}
	fmt.Println("  ╚═══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
