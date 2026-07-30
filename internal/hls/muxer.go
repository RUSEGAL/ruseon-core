package hls

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"

	"gritprofmediaserver/internal/buffer"
)

// Segment представляет один HLS ts-сегмент.
type Segment struct {
	Name     string
	Duration time.Duration
	Data     []byte
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
	go m.run()
	return m
}

func (m *Muxer) run() {
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

		// Если текущий сегмент достиг целевой длины и пришел новый I-кадр, закрываем сегмент
		if currentBuf != nil && frame.IsKeyFrame && frame.Timestamp-segmentStart >= m.targetDuration {
			m.mu.Lock()
			m.seqCount++
			seg := &Segment{
				Name:     fmt.Sprintf("stream_%d.ts", m.seqCount),
				Duration: frame.Timestamp - segmentStart,
				Data:     currentBuf.Bytes(),
			}
			m.segments = append(m.segments, seg)
			// Оставляем в памяти только последние 5 сегментов (окно Live в 10 секунд)
			if len(m.segments) > 5 {
				m.segments = m.segments[1:]
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
			currentBuf = &bytes.Buffer{}
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

// GetPlaylist генерирует M3U8 манифест.
func (m *Muxer) GetPlaylist() string {
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
		buf.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", seg.Duration.Seconds()))
		buf.WriteString(seg.Name + "\n")
	}

	return buf.String()
}

// GetSegment возвращает данные TS-сегмента по его имени.
func (m *Muxer) GetSegment(name string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, seg := range m.segments {
		if seg.Name == name {
			return seg.Data
		}
	}
	return nil
}
