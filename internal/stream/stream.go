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

// Stream представляет логику работы с конкретной камерой.
type Stream struct {
	ID  string
	URL string
	transport string

	ctx       context.Context
	cancelCtx context.CancelFunc

	// Буфер кадров для этой камеры
	ringBuffer *buffer.RingBuffer
	
	// Бродкастер метаданных
	metaBroadcaster *MetadataBroadcaster
	
	muxerMu        sync.Mutex
	hlsMuxer       *hls.Muxer
	lastHLSRequest time.Time
	lazyHLS        bool

	mp4Recorder *recorder.Recorder

	// Статистика
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
	
	sfGroup    singleflight.Group

	// Cached bound metrics for hot path
	metricNetRxBytes prometheus.Counter
	metricFramesRx   prometheus.Counter
	metricKeyFrames  prometheus.Counter

	currentBitrate atomic.Uint64
	isDegraded     atomic.Bool

	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewStream создает и запускает поток.
func NewStream(id, url string, record bool, lazyHLS bool, transport string) *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	rb := buffer.NewRingBuffer(100)
	rb.SetCameraID(id)
	s := &Stream{
		ID:         id,
		URL:        url,
		transport:  transport,
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
	} else {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.lazyHLSWatchdog()
		}()
	}

	if record {
		s.mp4Recorder = recorder.NewRecorder(id, rb, "recordings", func(degraded bool) {
			s.isDegraded.Store(degraded)
		})
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.bitrateTask()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run()
	}()

	return s
}

func (s *Stream) bitrateTask() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("id", s.ID).Msg("Recovered from panic in Stream.bitrateTask")
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastBytes uint64
	lastTime := time.Now()
	for {
		select {
		case <-s.ctx.Done():
			return
		case t := <-ticker.C:
			currentBytes := s.bytesReceived.Load()
			diff := currentBytes - lastBytes
			dt := t.Sub(lastTime).Seconds()
			if dt > 0 {
				bps := float64(diff) * 8 / dt
				s.currentBitrate.Store(uint64(bps))
			}
			lastBytes = currentBytes
			lastTime = t
		}
	}
}

// run выполняет бесконечный цикл подключения с ретраями.
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
			// Считаем объем байт (примерно, только полезная нагрузка NALU)
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
				// Prevent multiple concurrent 'camera_online' events by using CompareAndSwap
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

			// Оборачиваем в Frame и пишем в Ring Buffer
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
		// Add some jitter (e.g. 0 to 500ms)
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

// Stop останавливает работу потока, ожидает завершения всех фоновых воркеров.
// Метод является идемпотентным (защищен через sync.Once).
func (s *Stream) Stop() {
	s.stopOnce.Do(func() {
		s.cancelCtx()
		
		s.rtspMu.Lock()
		if s.rtspClient != nil {
			s.rtspClient.Close()
		}
		s.rtspMu.Unlock()

		s.ringBuffer.Close()
		
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

// PipelineConfig определяет параметры, управляющие жизненным циклом и поведением рантайм-пайплайна.
type PipelineConfig struct {
	URL       string
	Record    bool
	LazyHLS   bool
	Transport string
}

// MatchesPipelineConfig проверяет соответствие текущего состояния стрима заданной конфигурации пайплайна.
func (s *Stream) MatchesPipelineConfig(cfg PipelineConfig) bool {
	hasRecorder := s.mp4Recorder != nil
	return s.URL == cfg.URL && s.transport == cfg.Transport && s.lazyHLS == cfg.LazyHLS && hasRecorder == cfg.Record
}

// MatchesConfig проверяет, совпадают ли текущие параметры стрима с переданными.
func (s *Stream) MatchesConfig(url string, record, lazyHLS bool, transport string) bool {
	return s.MatchesPipelineConfig(PipelineConfig{
		URL:       url,
		Record:    record,
		LazyHLS:   lazyHLS,
		Transport: transport,
	})
}

// GetRingBuffer возвращает кольцевой буфер потока для чтения.
func (s *Stream) GetRingBuffer() *buffer.RingBuffer {
	return s.ringBuffer
}

func (s *Stream) GetMetadataBroadcaster() *MetadataBroadcaster {
	return s.metaBroadcaster
}

// WakeUpHLSMuxer возвращает мультиплексор HLS, просыпая его при необходимости.
func (s *Stream) WakeUpHLSMuxer() *hls.Muxer {
	v, _, _ := s.sfGroup.Do("wakeup", func() (interface{}, error) {
		s.muxerMu.Lock()
		defer s.muxerMu.Unlock()
		
		s.lastHLSRequest = time.Now()
		
		if s.hlsMuxer == nil {
			log.Info().Str("id", s.ID).Msg("Waking up HLS Muxer (Lazy Mode)")
			sub := s.metaBroadcaster.Subscribe()
			s.hlsMuxer = hls.NewMuxer(s.ID, s.ringBuffer, sub.C, func() { s.metaBroadcaster.Unsubscribe(sub) })
		}
		return s.hlsMuxer, nil
	})
	
	// Если мы не внутри Do, обновим время запроса для watchdog
	s.muxerMu.Lock()
	s.lastHLSRequest = time.Now()
	s.muxerMu.Unlock()

	return v.(*hls.Muxer)
}

func (s *Stream) lazyHLSWatchdog() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("id", s.ID).Msg("Recovered from panic in Stream.lazyHLSWatchdog")
		}
	}()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.muxerMu.Lock()
			if s.hlsMuxer != nil && time.Since(s.lastHLSRequest) > 60*time.Second {
				log.Info().Str("id", s.ID).Msg("Stopping HLS Muxer due to inactivity (Lazy Mode)")
				s.hlsMuxer.Stop()
				s.hlsMuxer = nil
			}
			s.muxerMu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// SetState updates the camera state.
func (s *Stream) SetState(st models.CameraState) {
	s.state.Store(st)
}

// AddBytesReceived increases received bytes counter.
func (s *Stream) AddBytesReceived(n uint64) {
	s.bytesReceived.Add(n)
	s.metricNetRxBytes.Add(float64(n))
}

// AddFramesReceived increases received frames counter and updates last frame time.
func (s *Stream) AddFramesReceived(n uint64) {
	s.framesReceived.Add(n)
	s.metricFramesRx.Add(float64(n))
	s.lastFrameTime.Store(time.Now().Unix())
}

// AddKeyFramesReceived increases received key frames counter and updates last key time.
func (s *Stream) AddKeyFramesReceived(n uint64) {
	s.keyFramesReceived.Add(n)
	s.metricKeyFrames.Add(float64(n))
	s.lastKeyTime.Store(time.Now().Unix())
}

// SetConnectedAt sets connection timestamp.
func (s *Stream) SetConnectedAt(t time.Time) {
	s.connectedAt.Store(t.Unix())
}

// AddBytesSent увеличивает счетчик исходящего трафика
func (s *Stream) AddBytesSent(n uint64) {
	s.bytesSent.Add(n)
}

// GetStats возвращает текущую статистику потока.
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
