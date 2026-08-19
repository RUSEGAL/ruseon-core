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
//   - EventBus       : Floods system events (camera_online, storage_warning); webhook sink
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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	cntIngestFrames atomic.Uint64
	cntIngestKeys   atomic.Uint64
	cntIngestBytes  atomic.Uint64

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

	// WebRTC Delivery
	cntWebRTCSessionsOK  atomic.Uint64
	cntWebRTCSessionsErr atomic.Uint64
	cntWebRTCRTPPackets  atomic.Uint64
	cntWebRTCBytesRx     atomic.Uint64

	// gRPC
	cntGRPCFrames    atomic.Uint64
	cntGRPCMeta      atomic.Uint64
	cntGRPCErr       atomic.Uint64
	cntGRPCBytesSent atomic.Uint64

	// EventBus
	cntEventsPub atomic.Uint64
	cntWebhooks  atomic.Uint64
)

// ─────────────────────────────────────────────────────────────────────────────
// /dev/null BlobStore
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
// In-memory StateStore
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

func (s *loadtestStore) Sync() error {
	return nil
}

func (s *loadtestStore) MigrateFromConfig(*config.Config) error { return nil }

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
		return &models.User{Username: username, Role: models.RoleAdmin}, nil
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

func (s *loadtestStore) ExportJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.cameras)
}

func (s *loadtestStore) ImportJSON([]byte) error      { return nil }
func (s *loadtestStore) BackupBadger(io.Writer) error { return nil }
func (s *loadtestStore) Close() error                 { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// Frame Payloads
// ─────────────────────────────────────────────────────────────────────────────

var (
	syntheticSPS = []byte{
		0x67, 0x42, 0x00, 0x1f, 0x96, 0x54, 0x05, 0x01,
		0xed, 0x80, 0xa8, 0x40, 0x00, 0x00, 0x03, 0x00,
		0x40, 0x00, 0x00, 0x0f, 0x03, 0xc6, 0x0c, 0x65, 0x80,
	}
	syntheticPPS = []byte{0x68, 0xce, 0x3c, 0x80}

	payloadIFrame = make([]byte, 80_000) // ~80 KB keyframe
	payloadPFrame = make([]byte, 8_000)  // ~8 KB P-frame
)

// ─────────────────────────────────────────────────────────────────────────────
// Layer 1: Synthetic RTSP Ingest
// ─────────────────────────────────────────────────────────────────────────────

func runSyntheticCamera(ctx context.Context, camID string, manager *stream.Manager) {
	st, ok := manager.GetStream(camID)
	if !ok {
		return
	}
	rb := st.GetRingBuffer()
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
		req.Header.Set("Authorization", "Bearer "+token)

		t0 := time.Now()
		resp, err := client.Do(req)
		lat := time.Since(t0)

		if err != nil {
			if ctx.Err() == nil {
				cntHTTPErr.Add(1)
			}
		} else {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			apiLatencySampler.Add(lat)

			switch {
			case resp.StatusCode >= 200 && resp.StatusCode < 300:
				cntHTTP2xx.Add(1)
				cntHTTPOK.Add(1)
				cntHTTPBytesSent.Add(uint64(len(data)))
			case resp.StatusCode >= 400 && resp.StatusCode < 500:
				cntHTTP4xx.Add(1)
				cntHTTPErr.Add(1)
			default:
				cntHTTP5xx.Add(1)
				cntHTTPErr.Add(1)
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
// Layer 3: HLS fMP4 Viewers
// ─────────────────────────────────────────────────────────────────────────────

func runHLSViewer(ctx context.Context, baseURL string, camIDs []string, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	camID := camIDs[randIntn(len(camIDs))]
	seenSegments := make(map[string]bool)

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

					segData, _ := io.ReadAll(segResp.Body)
					_ = segResp.Body.Close()

					if segResp.StatusCode == http.StatusOK {
						hlsSegmentSampler.Add(time.Since(tSeg))
						cntHLSSegmentsOK.Add(1)
						cntHLSBytesSent.Add(uint64(len(segData)))
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

	var pc *webrtc.PeerConnection
	var err error
	if engine != nil {
		pc, err = engine.NewPeerConnection(webrtc.Configuration{})
	} else {
		m := &webrtc.MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			cntWebRTCSessionsErr.Add(1)
			return
		}
		apiEngine := webrtc.NewAPI(webrtc.WithMediaEngine(m))
		pc, err = apiEngine.NewPeerConnection(webrtc.Configuration{})
	}
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

	<-ctx.Done()
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 5: gRPC StreamFrames Consumer
// ─────────────────────────────────────────────────────────────────────────────

func runGRPCFrameConsumer(ctx context.Context, addr, camID string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cntGRPCErr.Add(1)
		return
	}
	defer conn.Close()

	grpcStream, err := pb.NewFrameServiceClient(conn).StreamFrames(ctx, &pb.StreamRequest{CameraId: camID})
	if err != nil {
		cntGRPCErr.Add(1)
		return
	}

	for {
		t0 := time.Now()
		resp, err := grpcStream.Recv()
		if err != nil {
			return // Context cancelled or stream finished
		}
		grpcStreamSampler.Add(time.Since(t0))
		cntGRPCFrames.Add(1)
		if resp != nil {
			cntGRPCBytesSent.Add(uint64(len(resp.Payload)))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Layer 6: gRPC PushMetadata AI Pusher
// ─────────────────────────────────────────────────────────────────────────────

func runGRPCMetaPusher(ctx context.Context, addr string, camIDs []string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	ticker := time.NewTicker(100 * time.Millisecond) // 10 pushes/sec
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

	fmt.Printf(
		"[%4.0fs] Ingest=%d (fps=%.0f) | API_OK=%d Err=%d (p50=%.1fms, p95=%.1fms) | HLS_Segs=%d | WebRTC_Pkts=%d | gRPC_FPS=%.0f (meta=%d, err=%d) | Webhooks=%d | Heap=%dMB | Goroutines=%d\n",
		elapsed.Seconds(),
		fi, float64(fi)/elapsed.Seconds(),
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
// main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
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

	var wg sync.WaitGroup
	start := time.Now()
	fmt.Println("[run] Starting full-stack load test...")

	// Layer 1: Ingest (one synthetic camera per cameraID)
	for _, id := range camIDs {
		go runSyntheticCamera(ctx, id, manager)
	}

	// Warm-up: wait for first keyframes to populate RingBuffers
	time.Sleep(400 * time.Millisecond)

	// Layer 2: REST API Workers
	for i := 0; i < apiWorkers; i++ {
		wg.Add(1)
		go runAPIWorker(ctx, httpSrv.URL, adminToken, &wg)
	}

	// Layer 3: HLS fMP4 Viewers
	for i := 0; i < hlsViewers; i++ {
		wg.Add(1)
		go runHLSViewer(ctx, httpSrv.URL, camIDs, &wg)
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

	var totalDrops uint64
	for _, st := range manager.GetStreams() {
		// Collect drops from stream ring buffer if any
		_ = st
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
		},
		Ingest: IngestMetrics{
			TotalFrames: ingestFrames,
			KeyFrames:   cntIngestKeys.Load(),
			TotalBytes:  ingestBytes,
			FPS:         math.Round(float64(ingestFrames)/sec*10) / 10,
			BitrateMbps: math.Round(ingestMbps*10) / 10,
			Drops:       totalDrops,
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

func printFinalDashboard(r *BenchmarkResult) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                  FINAL BENCHMARK REPORT                                      ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Elapsed: %-8.1fs   CPU Cores: %-4d   Goroutines: %-5d   Heap: %-4dMB   Sys: %-5dMB     ║\n",
		r.DurationSec, r.System.NumCPU, r.System.GoroutinesEnd, r.System.HeapAllocMB, r.System.SysMB)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  [Ingest (30 FPS)]  Frames: %-8d  Keyframes: %-7d  FPS: %-6.0f  Bandwidth: %-5.1f Mbps     ║\n",
		r.Ingest.TotalFrames, r.Ingest.KeyFrames, r.Ingest.FPS, r.Ingest.BitrateMbps)
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
	fmt.Printf("║  [EventBus]         Delivered Webhooks: %-8d (Rate: %.0f/sec)                              ║\n",
		r.EventBus.Delivered, r.EventBus.RatePerSec)
	if r.Storage.RealDisk {
		fmt.Printf("║  [Disk I/O Storage] Files: %-5d   Write Rate: %-5.2f MB/s                                   ║\n",
			r.Storage.FilesCreated, r.Storage.WriteMBs)
	}
	fmt.Printf("║  [GC Telemetry]     Cycles: %-5d   Pause Total: %-6.2f ms   Max Pause: %-5.2f ms             ║\n",
		r.System.NumGC, r.System.GCPauseTotal, r.System.GCPauseMax)
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
	fmt.Println("  ╚═══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
