package hls

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/buffer"
)

// Segment представляет один HLS ts-сегмент.
type Segment struct {
	Name            string
	Duration        time.Duration
	Data            []byte
	buf             *bytes.Buffer // ссылка для возврата в пул
	IsDiscontinuity bool
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024*1024))
	},
}

// Muxer читает кадры из RingBuffer и упаковывает их в HLS (MPEG-TS сегменты).
type Muxer struct {
	streamID       string
	ringBuffer     *buffer.RingBuffer
	ctx            context.Context
	cancel         context.CancelFunc
	targetDuration time.Duration

	mu       sync.RWMutex
	segments []*Segment
	seqCount uint64

	lastFrameTime      atomic.Int64
	needsDiscontinuity bool
}

// NewMuxer создает новый Muxer для потока.
func NewMuxer(streamID string, rb *buffer.RingBuffer) *Muxer {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Muxer{
		streamID:       streamID,
		ringBuffer:     rb,
		ctx:            ctx,
		cancel:         cancel,
		targetDuration: 2 * time.Second, // Целевая длина сегмента: 2 секунды (хорошо для low-latency)
		segments:       make([]*Segment, 0, 10),
	}
	m.lastFrameTime.Store(time.Now().UnixNano())
	go m.run()
	go m.watchdog()
	return m
}

func (m *Muxer) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("streamID", m.streamID).Msg("Recovered from panic in Muxer.run")
		}
	}()
	reader := m.ringBuffer.NewReader()

	var currentBuf *bytes.Buffer
	var tsWriter *mpegts.Writer
	var track *mpegts.Track
	var segmentStart time.Duration

	// Читаем параметры кодека из буфера
	vps, sps, pps := m.ringBuffer.GetParams()

	for {
		if m.ctx.Err() != nil {
			break
		}

		if sps == nil || pps == nil {
			vps, sps, pps = m.ringBuffer.GetParams()
		}

		frame := reader.Read()
		if frame == nil {
			break
		}

		m.lastFrameTime.Store(time.Now().UnixNano())

		// Если текущий сегмент достиг целевой длины и пришел новый I-кадр, закрываем сегмент
		if currentBuf != nil && frame.IsKeyFrame && frame.Timestamp-segmentStart >= m.targetDuration {
			m.mu.Lock()
			m.seqCount++
			seg := &Segment{
				Name:            fmt.Sprintf("stream_%d.ts", m.seqCount),
				Duration:        frame.Timestamp - segmentStart,
				Data:            currentBuf.Bytes(),
				buf:             currentBuf,
				IsDiscontinuity: m.needsDiscontinuity,
			}
			m.needsDiscontinuity = false
			m.segments = append(m.segments, seg)
			// Оставляем в памяти только последние 5 сегментов (окно Live в 10 секунд)
			if len(m.segments) > 5 {
				oldSeg := m.segments[0]
				m.segments = m.segments[1:]
				if oldSeg.buf != nil {
					oldSeg.buf.Reset()
					bufferPool.Put(oldSeg.buf)
				}
			}
			m.mu.Unlock()

			currentBuf = nil
		}

		// Если нет активного буфера (начало или только что закрыли предыдущий сегмент)
		if currentBuf == nil {
			// Начинаем только с ключевого кадра
			if !frame.IsKeyFrame {
				continue
			}
			// Берем буфер из пула, избегая частых аллокаций гигантских слайсов
			currentBuf = bufferPool.Get().(*bytes.Buffer)
			var t mpegts.Codec
			if vps != nil {
				t = &mpegts.CodecH265{}
			} else {
				t = &mpegts.CodecH264{}
			}
			track = &mpegts.Track{
				Codec: t,
			}
			// NewWriter автоматически запишет PAT/PMT таблицы
			tsWriter = mpegts.NewWriter(currentBuf, []*mpegts.Track{track})
			segmentStart = frame.Timestamp
		}

		nalus := frame.NALUs

		// Для H264/H265 перед ключевым кадром добавляем параметры (VPS/SPS/PPS), 
		// чтобы клиенты могли инициализировать декодер, если они подключились к этому сегменту.
		if frame.IsKeyFrame && sps != nil && pps != nil {
			hasParams := false
			for _, n := range nalus {
				if len(n) > 0 {
					var typ uint8
					if vps != nil {
						typ = (n[0] >> 1) & 0x3F
						if typ == 32 || typ == 33 || typ == 34 { // VPS, SPS, PPS for HEVC
							hasParams = true
							break
						}
					} else {
						typ = n[0] & 0x1F
						if typ == 7 || typ == 8 { // SPS, PPS for H264
							hasParams = true
							break
						}
					}
				}
			}
			if !hasParams {
				if vps != nil {
					nalus = append([][]byte{vps, sps, pps}, nalus...)
				} else {
					nalus = append([][]byte{sps, pps}, nalus...)
				}
			}
		}

		// Переводим duration в 90kHz тики (стандарт для MPEG-TS)
		pts := int64(frame.Timestamp * 90000 / time.Second)

		// Пишем NALU в TS
		// Так как мы не имеем B-кадров от большинства IP камер, PTS == DTS.
		err := tsWriter.WriteH26x(track, pts, pts, frame.IsKeyFrame, nalus)
		if err != nil {
			// При записи в memory buffer ошибка практически невозможна
			continue
		}
	}
}

// Stop останавливает работу Muxer'а.
func (m *Muxer) Stop() {
	m.cancel()
}

// watchdog следит за поступлением новых кадров. Если кадров нет больше 5 секунд (обрыв связи),
// он берет последний готовый сегмент и добавляет его дубликат в плейлист с флагом Discontinuity.
// Это заставляет сторонние плееры (VLC, OBS) "заморозить" последний кадр и не вылетать по таймауту.
func (m *Muxer) watchdog() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("streamID", m.streamID).Msg("Recovered from panic in Muxer.watchdog")
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			lastTime := m.lastFrameTime.Load()
			// Если кадра не было 5 секунд
			if lastTime > 0 && time.Since(time.Unix(0, lastTime)) > 5*time.Second {
				if len(m.segments) > 0 {
					lastSeg := m.segments[len(m.segments)-1]
					m.seqCount++
					seg := &Segment{
						Name:            fmt.Sprintf("stream_%d.ts", m.seqCount),
						Duration:        lastSeg.Duration,
						Data:            lastSeg.Data,
						IsDiscontinuity: true, // Сигнал для плеера, что PTS может прыгнуть
					}
					m.segments = append(m.segments, seg)
					if len(m.segments) > 5 {
						m.segments = m.segments[1:]
					}
					// Сбрасываем таймер, чтобы следующий дубликат добавился еще через 5 секунд
					m.lastFrameTime.Store(time.Now().UnixNano())
					// Следующий РЕАЛЬНЫЙ сегмент (когда связь восстановится) тоже должен начаться с разрыва,
					// так как его PTS будет отличаться от PTS дубликата.
					m.needsDiscontinuity = true
				}
			}
			m.mu.Unlock()
		}
	}
}

// GetPlaylist генерирует M3U8 манифест.
func (m *Muxer) GetPlaylist() string {
	// В режиме Lazy HLS при первом запуске сегментов еще нет. 
	// Ждем до 3 секунд, пока сгенерируется хотя бы один сегмент, 
	// чтобы плееры (VLC) не зависали из-за пустого плейлиста.
	for i := 0; i < 60; i++ {
		m.mu.RLock()
		count := len(m.segments)
		m.mu.RUnlock()
		if count > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var buf bytes.Buffer
	buf.WriteString("#EXTM3U\n")
	buf.WriteString("#EXT-X-VERSION:3\n")
	
	maxDuration := 2
	for _, seg := range m.segments {
		d := int(seg.Duration.Seconds())
		if d > maxDuration {
			maxDuration = d
		}
	}
	buf.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", maxDuration+1))
	
	if len(m.segments) > 0 {
		seq := m.seqCount - uint64(len(m.segments)) + 1
		buf.WriteString(fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", seq))
	}

	for _, seg := range m.segments {
		if seg.IsDiscontinuity {
			buf.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		buf.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", seg.Duration.Seconds()))
		buf.WriteString(seg.Name)
		buf.WriteByte('\n')
	}

	return buf.String()
}

// GetSegment возвращает данные TS-сегмента по его имени.
func (m *Muxer) GetSegment(name string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, seg := range m.segments {
		if seg.Name == name {
			// Создаем копию байтов для ответа, так как buf может уйти обратно в пул.
			// Плеер читает медленно, и буфер может переиспользоваться во время отдачи файла.
			// Либо мы можем отдавать его как есть, но это риск Data Race. Для безопасности копируем.
			resp := make([]byte, len(seg.Data))
			copy(resp, seg.Data)
			return resp
		}
	}
	return nil
}
