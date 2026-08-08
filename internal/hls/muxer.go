package hls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

// Segment представляет один HLS ts-сегмент.
type Segment struct {
	Name            string
	Duration        time.Duration
	Data            []byte
	buf             *bytes.Buffer // ссылка для возврата в пул
	IsDiscontinuity bool
	VTTData         []byte
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

	metaChan    <-chan *pb.MetadataRequest
	unsubscribe func()

	mu       sync.RWMutex
	segments []*Segment
	seqCount uint64

	currentMetaMu sync.Mutex
	currentMeta   []*pb.MetadataRequest

	lastFrameTime      atomic.Int64
	needsDiscontinuity bool
}

// NewMuxer создает новый Muxer для потока.
func NewMuxer(streamID string, rb *buffer.RingBuffer, metaChan <-chan *pb.MetadataRequest, unsubscribe func()) *Muxer {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Muxer{
		streamID:       streamID,
		ringBuffer:     rb,
		ctx:            ctx,
		cancel:         cancel,
		targetDuration: 2 * time.Second, // Целевая длина сегмента: 2 секунды (хорошо для low-latency)
		segments:       make([]*Segment, 0, 10),
		metaChan:       metaChan,
		unsubscribe:    unsubscribe,
	}
	m.lastFrameTime.Store(time.Now().UnixNano())
	go m.run()
	go m.watchdog()
	if metaChan != nil {
		go m.readMetadata()
	}
	return m
}

func (m *Muxer) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("streamID", m.streamID).Msg("Recovered from panic in Muxer.run")
		}
	}()
	reader := m.ringBuffer.NewReader()
	defer reader.Close()

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
			m.currentMetaMu.Lock()
			meta := m.currentMeta
			m.currentMeta = nil
			m.currentMetaMu.Unlock()

			segmentStartPts := int64(segmentStart * 90000 / time.Second)
			var vttBuf bytes.Buffer
			vttBuf.WriteString("WEBVTT\n")
			vttBuf.WriteString(fmt.Sprintf("X-TIMESTAMP-MAP=MPEGTS:%d,LOCAL:00:00:00.000\n\n", segmentStartPts))

			for _, req := range meta {
				var localPts int64
				if int64(req.Pts) > segmentStartPts {
					localPts = int64(req.Pts) - segmentStartPts
				}

				ms := localPts / 90
				sec := ms / 1000
				ms = ms % 1000
				min := sec / 60
				sec = sec % 60
				hr := min / 60
				min = min % 60

				// Сделаем cue длительностью 300мс
				msEnd := ms + 300
				secEnd := sec
				if msEnd >= 1000 {
					msEnd -= 1000
					secEnd++
				}
				minEnd := min
				if secEnd >= 60 {
					secEnd -= 60
					minEnd++
				}
				hrEnd := hr
				if minEnd >= 60 {
					minEnd -= 60
					hrEnd++
				}

				vttBuf.WriteString(fmt.Sprintf("%02d:%02d:%02d.%03d --> %02d:%02d:%02d.%03d\n", hr, min, sec, ms, hrEnd, minEnd, secEnd, msEnd))
				data, _ := json.Marshal(req)
				vttBuf.Write(data)
				vttBuf.WriteString("\n\n")
			}

			m.mu.Lock()
			m.seqCount++
			seg := &Segment{
				Name:            fmt.Sprintf("stream_%d.ts", m.seqCount),
				Duration:        frame.Timestamp - segmentStart,
				Data:            currentBuf.Bytes(),
				buf:             currentBuf,
				IsDiscontinuity: m.needsDiscontinuity,
				VTTData:         vttBuf.Bytes(),
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

func (m *Muxer) readMetadata() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case req, ok := <-m.metaChan:
			if !ok {
				return
			}
			m.currentMetaMu.Lock()
			m.currentMeta = append(m.currentMeta, req)
			m.currentMetaMu.Unlock()
		}
	}
}

// Stop останавливает работу Muxer'а.
func (m *Muxer) Stop() {
	if m.unsubscribe != nil {
		m.unsubscribe()
	}
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
	// Ждем до 5 секунд, пока сгенерируется хотя бы один сегмент, 
	// чтобы плееры (VLC, k6) не зависали и не паниковали из-за пустого плейлиста.
	for i := 0; i < 100; i++ {
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

// GetSubsPlaylist генерирует M3U8 манифест для субтитров (WebVTT).
func (m *Muxer) GetSubsPlaylist() string {
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
		// Меняем .ts на .vtt
		vttName := seg.Name[:len(seg.Name)-3] + ".vtt"
		buf.WriteString(vttName)
		buf.WriteByte('\n')
	}

	return buf.String()
}

// GetSegment возвращает данные TS или VTT сегмента по его имени.
func (m *Muxer) GetSegment(name string) ([]byte, string) {
	isVTT := len(name) > 4 && name[len(name)-4:] == ".vtt"
	tsName := name
	if isVTT {
		tsName = name[:len(name)-4] + ".ts"
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, seg := range m.segments {
		if seg.Name == tsName {
			if isVTT {
				return seg.VTTData, "text/vtt"
			}
			resp := make([]byte, len(seg.Data))
			copy(resp, seg.Data)
			return resp, "video/mp2t"
		}
	}
	return nil, ""
}
