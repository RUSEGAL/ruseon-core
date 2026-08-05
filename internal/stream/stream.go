package stream

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/internal/hls"
	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/internal/recorder"
	"github.com/RUSEGAL/ruseon-core/internal/rtsp"
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
	
	muxerMu        sync.Mutex
	hlsMuxer       *hls.Muxer
	lastHLSRequest time.Time
	lazyHLS        bool

	mp4Recorder *recorder.Recorder

	// Статистика
	connected         atomic.Bool
	connectedAt       atomic.Int64
	bytesReceived     atomic.Uint64
	bytesSent         atomic.Uint64
	framesReceived    atomic.Uint64
	keyFramesReceived atomic.Uint64
	reconnects        atomic.Uint64

	rtspClient *rtsp.Client
	rtspMu     sync.Mutex
	
	sfGroup    singleflight.Group
}

// NewStream создает и запускает поток.
func NewStream(id, url string, record bool, lazyHLS bool, transport string) *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	rb := buffer.NewRingBuffer(100)
	s := &Stream{
		ID:         id,
		URL:        url,
		transport:  transport,
		ctx:        ctx,
		cancelCtx:  cancel,
		// Буфер на 100 кадров (примерно 4 секунды при 25 FPS)
		ringBuffer: rb,
		lazyHLS:    lazyHLS,
	}

	if !lazyHLS {
		s.hlsMuxer = hls.NewMuxer(id, rb)
	} else {
		go s.lazyHLSWatchdog()
	}

	if record {
		s.mp4Recorder = recorder.NewRecorder(id, rb, "recordings")
	}

	go s.run()

	return s
}

// run выполняет бесконечный цикл подключения с ретраями.
func (s *Stream) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("id", s.ID).Msg("Recovered from panic in Stream.run")
		}
	}()
	log.Info().Str("id", s.ID).Msg("Starting stream processing")
	for {
		if s.ctx.Err() != nil {
			break
		}

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
			s.bytesReceived.Add(uint64(size)) //nolint:gosec
			
			if !s.connected.Load() {
				s.connected.Store(true)
				s.connectedAt.Store(time.Now().Unix())
				log.Info().Str("id", s.ID).Msg("RTSP connected and receiving frames")
			}
			
			s.framesReceived.Add(1)
			if isKeyFrame {
				s.keyFramesReceived.Add(1)
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

		s.connected.Store(false)
		s.reconnects.Add(1)

		if s.ctx.Err() != nil {
			log.Info().Str("id", s.ID).Msg("Stream stopped by context")
			break
		}

		log.Warn().Err(err).Str("id", s.ID).Msg("RTSP connection lost, reconnecting in 5s...")

		timer := time.NewTimer(5 * time.Second)
		select {
		case <-timer.C:
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
	}
}

// Stop останавливает работу потока.
func (s *Stream) Stop() {
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
}

// GetRingBuffer возвращает кольцевой буфер потока для чтения.
func (s *Stream) GetRingBuffer() *buffer.RingBuffer {
	return s.ringBuffer
}

// WakeUpHLSMuxer возвращает мультиплексор HLS, просыпая его при необходимости.
func (s *Stream) WakeUpHLSMuxer() *hls.Muxer {
	v, _, _ := s.sfGroup.Do("wakeup", func() (interface{}, error) {
		s.muxerMu.Lock()
		defer s.muxerMu.Unlock()
		
		s.lastHLSRequest = time.Now()
		
		if s.hlsMuxer == nil {
			log.Info().Str("id", s.ID).Msg("Waking up HLS Muxer (Lazy Mode)")
			s.hlsMuxer = hls.NewMuxer(s.ID, s.ringBuffer)
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

// AddBytesSent увеличивает счетчик исходящего трафика
func (s *Stream) AddBytesSent(n uint64) {
	s.bytesSent.Add(n)
}

// GetStats возвращает текущую статистику потока.
func (s *Stream) GetStats() models.CameraStats {
	var uptime int64
	if s.connected.Load() {
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

	return models.CameraStats{
		Connected:     s.connected.Load(),
		BytesReceived: s.bytesReceived.Load(),
		BytesSent:     s.bytesSent.Load(),
		Uptime:        uptime,
		Frames:        s.framesReceived.Load(),
		KeyFrames:     s.keyFramesReceived.Load(),
		Reconnects:    s.reconnects.Load(),
		Codec:         codec,
	}
}
