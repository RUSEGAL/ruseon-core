package stream

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"gritprofmediaserver/internal/buffer"
	"gritprofmediaserver/internal/hls"
	"gritprofmediaserver/internal/models"
	"gritprofmediaserver/internal/recorder"
	"gritprofmediaserver/internal/rtsp"
)

// Stream представляет логику работы с конкретной камерой.
type Stream struct {
	ID  string
	URL string

	ctx       context.Context
	cancelCtx context.CancelFunc

	// Буфер кадров для этой камеры
	ringBuffer *buffer.RingBuffer
	hlsMuxer   *hls.Muxer
	mp4Recorder *recorder.Recorder

	// Статистика
	connected         atomic.Bool
	connectedAt       time.Time
	bytesReceived     atomic.Uint64
	bytesSent         atomic.Uint64
	framesReceived    atomic.Uint64
	keyFramesReceived atomic.Uint64
}

// NewStream создает и запускает поток.
func NewStream(id, url string, record bool) *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	rb := buffer.NewRingBuffer(100)
	s := &Stream{
		ID:         id,
		URL:        url,
		ctx:        ctx,
		cancelCtx:  cancel,
		// Буфер на 100 кадров (примерно 4 секунды при 25 FPS)
		ringBuffer: rb,
		hlsMuxer:   hls.NewMuxer(id, rb),
	}

	if record {
		s.mp4Recorder = recorder.NewRecorder(id, rb, "recordings")
	}

	go s.run()

	return s
}

// run выполняет бесконечный цикл подключения с ретраями.
func (s *Stream) run() {
	log.Info().Str("id", s.ID).Msg("Starting stream processing")
	for {
		if s.ctx.Err() != nil {
			break
		}

		client := rtsp.NewClient(s.URL)

		log.Info().Str("id", s.ID).Str("url", s.URL).Msg("Connecting to RTSP")
		
		err := client.Start(s.ctx, func(nalus [][]byte, pts time.Duration, isKeyFrame bool) {
			// Считаем объем байт (примерно, только полезная нагрузка NALU)
			size := 0
			for _, n := range nalus {
				size += len(n)
			}
			s.bytesReceived.Add(uint64(size))
			
			if !s.connected.Load() {
				s.connected.Store(true)
				s.connectedAt = time.Now()
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

		if s.ctx.Err() != nil {
			log.Info().Str("id", s.ID).Msg("Stream stopped by context")
			break
		}

		log.Warn().Err(err).Str("id", s.ID).Msg("RTSP connection lost, reconnecting in 5s...")

		select {
		case <-time.After(5 * time.Second):
		case <-s.ctx.Done():
			break
		}
	}
}

// Stop останавливает работу потока.
func (s *Stream) Stop() {
	s.cancelCtx()
	s.ringBuffer.Close()
	s.hlsMuxer.Stop()
	if s.mp4Recorder != nil {
		s.mp4Recorder.Stop()
	}
}

// GetRingBuffer возвращает кольцевой буфер потока для чтения.
func (s *Stream) GetRingBuffer() *buffer.RingBuffer {
	return s.ringBuffer
}

// GetHLSMuxer возвращает мультиплексор HLS.
func (s *Stream) GetHLSMuxer() *hls.Muxer {
	return s.hlsMuxer
}

// AddBytesSent увеличивает счетчик исходящего трафика
func (s *Stream) AddBytesSent(n uint64) {
	s.bytesSent.Add(n)
}

// GetStats возвращает текущую статистику потока.
func (s *Stream) GetStats() models.CameraStats {
	var uptime int64
	if s.connected.Load() {
		uptime = int64(time.Since(s.connectedAt).Seconds())
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
		Codec:         codec,
	}
}
