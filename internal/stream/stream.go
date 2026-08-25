// Package stream implements high-level camera stream lifecycle management, reconnection backoff loops,
// bandwidth telemetry calculations, lazy HLS activation, and metadata multiplexing.
package stream

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/internal/hls"
	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/internal/recorder"
	"github.com/RUSEGAL/ruseon-core/internal/rtsp"
	"github.com/RUSEGAL/ruseon-core/pkg/metrics"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// Stream manages the full lifecycle of an individual camera ingest session.
//
// Responsibilities:
//   - RTSP connection establishment and automatic exponential backoff reconnection (with jitter).
//   - Ingesting video frames into an internal buffer.RingBuffer.
//   - Optional continuous MP4 archive recording with GOP boundary flushing.
//   - On-demand (Lazy) or continuous HLS multiplexing.
//   - Collecting and exposing atomic real-time throughput, bitrate, frame counts, and health telemetry.
type Stream struct {
	// ID is the unique camera identifier.
	ID string
	// URL is the RTSP source stream URI.
	URL string
	transport string
	tokenAuth bool

	ctx       context.Context
	cancelCtx context.CancelFunc

	// Frame ring buffer
	ringBuffer *buffer.RingBuffer
	
	// AI metadata broadcaster
	metaBroadcaster *MetadataBroadcaster
	
	muxerMu        sync.Mutex
	hlsMuxer       *hls.Muxer
	lastHLSRequest atomic.Int64
	lazyHLS        bool

	mp4Recorder *recorder.Recorder

	// Atomic telemetry and state counters
	state             atomic.Value
	connectedAt       atomic.Int64
	bytesReceived     atomic.Uint64
	bytesSent         atomic.Uint64
	framesReceived    atomic.Uint64
	keyFramesReceived atomic.Uint64
	reconnects        atomic.Uint64
	lastFrameTime     atomic.Int64
	lastKeyTime       atomic.Int64
	lastError         atomic.Value

	rtspClient *rtsp.Client
	rtspMu     sync.Mutex
	
	sfGroup singleflight.Group

	// Cached bound metrics for hot path
	metricNetRxBytes prometheus.Counter
	metricFramesRx   prometheus.Counter
	metricKeyFrames  prometheus.Counter

	currentBitrate  atomic.Uint64
	lastBytes       atomic.Uint64
	lastBitrateCalc atomic.Int64
	isDegraded      atomic.Bool

	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewStream instantiates and launches a new camera Stream processing goroutine.
//
// Parameters:
//   - id: Unique camera identifier.
//   - url: RTSP source URI.
//   - record: If true, enables continuous fMP4 archive recording.
//   - lazyHLS: If true, defers HLS muxer initialization until a viewer requests a playlist.
//   - transport: RTSP transport ("tcp", "udp", or "auto").
//   - tokenAuth: If true, restricts media endpoints with short-lived stream token requirements.
func NewStream(id, url string, record bool, lazyHLS bool, transport string, tokenAuth bool) *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	rb := buffer.NewRingBuffer(100)
	rb.SetCameraID(id)
	s := &Stream{
		ID:         id,
		URL:        url,
		transport:  transport,
		tokenAuth:  tokenAuth,
		ctx:        ctx,
		cancelCtx:  cancel,
		ringBuffer: rb,
		metaBroadcaster: NewMetadataBroadcaster(),
		lazyHLS:    lazyHLS,
		metricNetRxBytes: metrics.NetworkReceiveBytesTotal.WithLabelValues(id),
		metricFramesRx:   metrics.FramesReceivedTotal.WithLabelValues(id),
		metricKeyFrames:  metrics.KeyFramesTotal.WithLabelValues(id),
	}
	s.state.Store(models.StateConnecting)
	s.lastError.Store("")

	if !lazyHLS {
		sub := s.metaBroadcaster.Subscribe()
		s.hlsMuxer = hls.NewMuxer(id, rb, sub.C, func() { s.metaBroadcaster.Unsubscribe(sub) })
	}

	if record {
		s.mp4Recorder = recorder.NewRecorder(id, rb, "recordings", func(degraded bool) {
			s.isDegraded.Store(degraded)
		})
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run()
	}()

	return s
}

// run manages the continuous connection loop with exponential backoff and jitter.
func (s *Stream) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("id", s.ID).Msg("Recovered from panic in Stream.run")
			s.state.Store(models.StateOffline)
			s.lastError.Store(fmt.Sprintf("panic: %v", r))
			s.isDegraded.Store(true)
		}
	}()

	if s.URL == "" || s.URL == "synthetic" || strings.HasPrefix(s.URL, "synthetic://") {
		log.Info().Str("id", s.ID).Msg("Synthetic stream started (in-memory ingest mode)")
		<-s.ctx.Done()
		return
	}

	log.Info().Str("id", s.ID).Msg("Starting stream processing")
	for s.ctx.Err() == nil {
		log.Info().Str("id", s.ID).Str("url", s.URL).Msg("Connecting to RTSP")

		s.rtspMu.Lock()
		s.rtspClient = rtsp.NewClient(s.ID, s.URL, s.transport)
		s.rtspMu.Unlock()
		
		err := s.rtspClient.Start(s.ctx, func(nalus [][]byte, pts time.Duration, isKeyFrame bool) {
			size := 0
			for _, n := range nalus {
				size += len(n)
			}
			// #nosec G115 -- size of frame payload is non-negative
			s.bytesReceived.Add(uint64(size))
			s.metricNetRxBytes.Add(float64(size))
			
			s.lastFrameTime.Store(time.Now().Unix())
			
			oldState, _ := s.state.Load().(models.CameraState)
			if oldState != models.StateOnline {
				if s.state.CompareAndSwap(oldState, models.StateOnline) {
					s.lastError.Store("")
					s.reconnects.Store(0) // Reset reconnect backoff
					s.connectedAt.Store(time.Now().Unix())
					metrics.ActiveStreams.WithLabelValues(s.ID).Inc()
					log.Info().Str("id", s.ID).Msg("RTSP connected and receiving frames")
					if registry.CurrentEventBus != nil {
						registry.CurrentEventBus.Publish("camera_online", s.ID, nil)
					}
				}
			}
			
			s.metricFramesRx.Inc()
			s.framesReceived.Add(1)
			if isKeyFrame {
				s.metricKeyFrames.Inc()
				s.keyFramesReceived.Add(1)
				s.lastKeyTime.Store(time.Now().Unix())
			}

			// Wrap in Frame and write to RingBuffer
			f := &buffer.Frame{
				Timestamp:  pts,
				IsKeyFrame: isKeyFrame,
				NALUs:      nalus,
			}
			s.ringBuffer.Write(f)
		}, func(vps, sps, pps []byte) {
			s.ringBuffer.SetParams(vps, sps, pps)
			log.Info().Str("id", s.ID).Msg("Received codec parameters")
		})

		oldState, _ := s.state.Load().(models.CameraState)
		if s.state.CompareAndSwap(oldState, models.StateOffline) && oldState == models.StateOnline {
			metrics.ActiveStreams.WithLabelValues(s.ID).Dec()
		}
		
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
			s.lastError.Store(errMsg)
		}
		if registry.CurrentEventBus != nil {
			registry.CurrentEventBus.Publish("camera_offline", s.ID, map[string]string{"error": errMsg})
		}
		s.reconnects.Add(1)
		metrics.StreamReconnectsTotal.WithLabelValues(s.ID).Inc()

		if s.ctx.Err() != nil {
			log.Info().Str("id", s.ID).Msg("Stream stopped by context")
			break
		}

		recs := s.reconnects.Load()
		if recs > 6 {
			recs = 6 // Cap at 64s before bitshift to prevent int overflow
		}
		backoffSec := int(1 << recs)
		
		if backoffSec > 60 {
			backoffSec = 60
		}
		if backoffSec < 1 {
			backoffSec = 1
		}
		// Add jitter between 0 and 500ms
		jitter := time.Duration(time.Now().UnixNano()%500) * time.Millisecond
		reconnectDelay := time.Duration(backoffSec)*time.Second + jitter

		log.Warn().Err(err).Str("id", s.ID).Dur("delay", reconnectDelay).Msg("RTSP connection lost, reconnecting...")

		timer := time.NewTimer(reconnectDelay)
		select {
		case <-timer.C:
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
	}
}

// Stop terminates stream ingest, closes RTSP connections, stops recorders and HLS muxers,
// and blocks until all background worker goroutines have exited.
//
// Thread-safety: Stop is idempotent and safe for concurrent execution protected by sync.Once.
func (s *Stream) Stop() {
	s.stopOnce.Do(func() {
		if s.cancelCtx != nil {
			s.cancelCtx()
		}
		
		s.rtspMu.Lock()
		if s.rtspClient != nil {
			s.rtspClient.Close()
		}
		s.rtspMu.Unlock()

		if s.ringBuffer != nil {
			s.ringBuffer.Close()
		}
		
		s.muxerMu.Lock()
		if s.hlsMuxer != nil {
			s.hlsMuxer.Stop()
		}
		s.muxerMu.Unlock()
		
		if s.mp4Recorder != nil {
			s.mp4Recorder.Stop()
		}

		s.wg.Wait()
	})
}

// PipelineConfig defines the declarative runtime pipeline settings of a camera stream.
type PipelineConfig struct {
	// URL is the RTSP source stream URI.
	URL string
	// Record determines whether MP4 archive recording is enabled.
	Record bool
	// LazyHLS determines whether HLS muxing is on-demand.
	LazyHLS bool
	// Transport is the RTSP transport ("tcp", "udp", or "auto").
	Transport string
	// TokenAuth specifies whether stream token verification is required.
	TokenAuth bool
}

// MatchesPipelineConfig returns true if the current active stream pipeline matches cfg.
func (s *Stream) MatchesPipelineConfig(cfg PipelineConfig) bool {
	hasRecorder := s.mp4Recorder != nil
	return s.URL == cfg.URL && s.transport == cfg.Transport && s.lazyHLS == cfg.LazyHLS && s.tokenAuth == cfg.TokenAuth && hasRecorder == cfg.Record
}

// MatchesConfig checks whether the stream's configuration parameters match the provided arguments.
func (s *Stream) MatchesConfig(url string, record, lazyHLS bool, transport string, tokenAuth bool) bool {
	return s.MatchesPipelineConfig(PipelineConfig{
		URL:       url,
		Record:    record,
		LazyHLS:   lazyHLS,
		Transport: transport,
		TokenAuth: tokenAuth,
	})
}

// IsTokenAuth reports whether this stream requires short-lived token authentication for media playback.
func (s *Stream) IsTokenAuth() bool {
	return s.tokenAuth
}

// GetRingBuffer returns the underlying frame ring buffer.
func (s *Stream) GetRingBuffer() *buffer.RingBuffer {
	return s.ringBuffer
}

// GetMetadataBroadcaster returns the AI metadata broadcaster associated with this stream.
func (s *Stream) GetMetadataBroadcaster() *MetadataBroadcaster {
	return s.metaBroadcaster
}

// WakeUpHLSMuxer returns the active HLS muxer instance, starting it on-demand if in LazyHLS mode.
// It uses singleflight to deduplicate concurrent startup requests.
func (s *Stream) WakeUpHLSMuxer() *hls.Muxer {
	s.lastHLSRequest.Store(time.Now().UnixNano())

	// Fast-path: if muxer is already active, return without singleflight or allocation overhead
	s.muxerMu.Lock()
	m := s.hlsMuxer
	s.muxerMu.Unlock()
	if m != nil {
		return m
	}

	// Slow-path: singleflight concurrent startup on first viewer request
	v, _, _ := s.sfGroup.Do("wakeup", func() (interface{}, error) {
		s.muxerMu.Lock()
		defer s.muxerMu.Unlock()
		
		if s.hlsMuxer == nil {
			log.Info().Str("id", s.ID).Msg("Waking up HLS Muxer (Lazy Mode)")
			sub := s.metaBroadcaster.Subscribe()
			s.hlsMuxer = hls.NewMuxer(s.ID, s.ringBuffer, sub.C, func() { s.metaBroadcaster.Unsubscribe(sub) })
		}
		return s.hlsMuxer, nil
	})
	
	return v.(*hls.Muxer)
}

// TickHousekeeping executes 1-second periodic maintenance for the stream:
//   - Lock-free rolling bitrate calculation.
//   - Auto-stops inactive Lazy HLS muxers if no requests were received in 60s.
//   - Runs frame stall watchdog checks via the active HLS muxer.
func (s *Stream) TickHousekeeping(now time.Time) {
	// 1. Lock-free bitrate calculation
	currentBytes := s.bytesReceived.Load()
	lastBytes := s.lastBytes.Swap(currentBytes)
	lastTime := s.lastBitrateCalc.Swap(now.UnixNano())
	if lastTime > 0 {
		dt := float64(now.UnixNano()-lastTime) / 1e9
		if dt > 0 {
			diff := currentBytes - lastBytes
			bps := float64(diff) * 8 / dt
			s.currentBitrate.Store(uint64(bps))
		}
	}

	// 2. Stop inactive Lazy HLS Muxer (inactivity > 60 seconds)
	if s.lazyHLS {
		lastReq := s.lastHLSRequest.Load()
		if lastReq > 0 && now.Sub(time.Unix(0, lastReq)) > 60*time.Second {
			s.muxerMu.Lock()
			if s.hlsMuxer != nil {
				log.Info().Str("id", s.ID).Msg("Stopping HLS Muxer due to inactivity (Lazy Mode)")
				s.hlsMuxer.Stop()
				s.hlsMuxer = nil
			}
			s.muxerMu.Unlock()
		}
	}

	// 3. Delegate frame watchdog check to active HLS Muxer
	s.muxerMu.Lock()
	muxer := s.hlsMuxer
	s.muxerMu.Unlock()
	if muxer != nil {
		muxer.CheckWatchdog(now)
	}
}

// SetState updates the operational status of the camera stream.
func (s *Stream) SetState(st models.CameraState) {
	s.state.Store(st)
}

// AddBytesReceived increments the received byte counter and Prometheus receive metric.
func (s *Stream) AddBytesReceived(n uint64) {
	s.bytesReceived.Add(n)
	s.metricNetRxBytes.Add(float64(n))
}

// AddFramesReceived increments the total received frame counter and updates LastFrameTime.
func (s *Stream) AddFramesReceived(n uint64) {
	s.framesReceived.Add(n)
	s.metricFramesRx.Add(float64(n))
	s.lastFrameTime.Store(time.Now().Unix())
}

// AddKeyFramesReceived increments the received keyframe counter and updates LastKeyTime.
func (s *Stream) AddKeyFramesReceived(n uint64) {
	s.keyFramesReceived.Add(n)
	s.metricKeyFrames.Add(float64(n))
	s.lastKeyTime.Store(time.Now().Unix())
}

// SetConnectedAt updates the Unix timestamp of the latest successful stream connection.
func (s *Stream) SetConnectedAt(t time.Time) {
	s.connectedAt.Store(t.Unix())
}

// AddBytesSent increments the cumulative egress byte counter for viewers.
func (s *Stream) AddBytesSent(n uint64) {
	s.bytesSent.Add(n)
}

// GetStats compiles and returns a point-in-time snapshot of stream metrics and telemetry.
func (s *Stream) GetStats() models.CameraStats {
	var uptime int64
	var st models.CameraState
	if val, ok := s.state.Load().(models.CameraState); ok {
		st = val
	} else {
		st = models.StateOffline
	}

	if st == models.StateOnline {
		if s.isDegraded.Load() {
			st = models.StateDegraded
		}
		at := s.connectedAt.Load()
		if at > 0 {
			uptime = int64(time.Since(time.Unix(at, 0)).Seconds())
		}
	}
	
	codec := "-"
	if s.ringBuffer != nil {
		vps, sps, _ := s.ringBuffer.GetParams()
		if len(vps) > 0 {
			codec = "H.265 / HEVC"
		} else if len(sps) > 0 {
			codec = "H.264 / AVC"
		}
	}

	lastErrStr := ""
	if val, ok := s.lastError.Load().(string); ok {
		lastErrStr = val
	}

	return models.CameraStats{
		State:         st,
		BytesReceived: s.bytesReceived.Load(),
		BytesSent:     s.bytesSent.Load(),
		Uptime:        uptime,
		Frames:        s.framesReceived.Load(),
		KeyFrames:     s.keyFramesReceived.Load(),
		Reconnects:    s.reconnects.Load(),
		Codec:         codec,
		LastFrameTime: s.lastFrameTime.Load(),
		LastKeyTime:   s.lastKeyTime.Load(),
		LastError:     lastErrStr,
		Bitrate:       float64(s.currentBitrate.Load()),
	}
}

// GetState returns the current operational CameraState.
func (s *Stream) GetState() models.CameraState {
	if val, ok := s.state.Load().(models.CameraState); ok {
		return val
	}
	return models.StateOffline
}

// AddReconnects increments the reconnect counter by n.
func (s *Stream) AddReconnects(n uint64) {
	s.reconnects.Add(n)
}

// Context returns the lifecycle context of the stream.
func (s *Stream) Context() context.Context {
	return s.ctx
}


