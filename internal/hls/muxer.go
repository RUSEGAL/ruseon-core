package hls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

var emptyVTTData = []byte("WEBVTT\n\n")

// Segment представляет один HLS ts-сегмент.
type Segment struct {
	Name            string
	Duration        time.Duration
	Data            []byte
	buf             *bytes.Buffer // ссылка для возврата в пул
	refCount        atomic.Int32  // Reference counter для Zero-Copy
	IsDiscontinuity bool
	VTTData         []byte
}

// Retain увеличивает счетчик ссылок на сегмент.
func (s *Segment) Retain() {
	s.refCount.Add(1)
}

// Release уменьшает счетчик ссылок. Когда счетчик падает до 0, буфер возвращается в sync.Pool.
func (s *Segment) Release() {
	if s.refCount.Add(-1) == 0 {
		if s.buf != nil {
			s.buf.Reset()
			bufferPool.Put(s.buf)
			s.buf = nil
			s.Data = nil
		}
	}
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024*1024))
	},
}

var playlistPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 1024)
		return &buf
	},
}

// Muxer читает кадры из RingBuffer и упаковывает их в HLS (MPEG-TS сегменты).
type Muxer struct {
	streamID       string
	ringBuffer     *buffer.RingBuffer
	ctx            context.Context
	cancel         context.CancelFunc
	targetDuration time.Duration
	maxSegments    int
	wg             sync.WaitGroup

	metaChan    <-chan *pb.MetadataRequest
	unsubscribe func()

	mu       sync.RWMutex
	segments []*Segment
	seqCount uint64

	currentMeta []*pb.MetadataRequest

	lastFrameTime      atomic.Int64
	needsDiscontinuity bool

	firstSegReady chan struct{}
	firstSegOnce  sync.Once

	cachedPlaylist     atomic.Pointer[string]
	cachedSubsPlaylist atomic.Pointer[string]
}

// NewMuxer создает новый Muxer для потока с настраиваемым скользящим окном сегментов.
func NewMuxer(streamID string, rb *buffer.RingBuffer, metaChan <-chan *pb.MetadataRequest, unsubscribe func(), maxSegments ...int) *Muxer {
	ctx, cancel := context.WithCancel(context.Background())
	maxSegs := 3
	if len(maxSegments) > 0 && maxSegments[0] >= 3 {
		maxSegs = maxSegments[0]
	}

	m := &Muxer{
		streamID:       streamID,
		ringBuffer:     rb,
		ctx:            ctx,
		cancel:         cancel,
		targetDuration: 2 * time.Second, // Целевая длина сегмента: 2 секунды (хорошо для low-latency)
		maxSegments:    maxSegs,
		segments:       make([]*Segment, 0, maxSegs+2),
		metaChan:       metaChan,
		unsubscribe:    unsubscribe,
		firstSegReady:  make(chan struct{}),
	}
	m.lastFrameTime.Store(time.Now().UnixNano())
	m.wg.Add(1)
	go m.run()
	return m
}

func (m *Muxer) run() {
	defer m.wg.Done()
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

	metaChan := m.metaChan

	for {
		if sps == nil || pps == nil {
			vps, sps, pps = m.ringBuffer.GetParams()
		}

		var frame *buffer.Frame

		select {
		case <-m.ctx.Done():
			return
		case req, ok := <-metaChan:
			if !ok {
				metaChan = nil
				continue
			}
			if req != nil {
				m.currentMeta = append(m.currentMeta, req)
			}
			continue
		case f, ok := <-reader.C:
			if !ok || f == nil {
				return
			}
			frame = f
		}

		m.lastFrameTime.Store(time.Now().UnixNano())

		if frame.IsKeyFrame {
			newVps, newSps, newPps := m.ringBuffer.GetParams()
			if (!bytes.Equal(sps, newSps) || !bytes.Equal(pps, newPps) || !bytes.Equal(vps, newVps)) && newSps != nil {
				vps, sps, pps = newVps, newSps, newPps
				m.needsDiscontinuity = true
			}
		}

		// Если текущий сегмент достиг целевой длины и пришел новый I-кадр, закрываем сегмент
		if currentBuf != nil && frame.IsKeyFrame && frame.Timestamp-segmentStart >= m.targetDuration {
			meta := m.currentMeta
			m.currentMeta = nil

			var vttData []byte
			if len(meta) == 0 {
				vttData = emptyVTTData
			} else {
				segmentStartPts := int64(segmentStart * 90000 / time.Second)
				var vttBuf bytes.Buffer
				vttBuf.WriteString("WEBVTT\n")
				fmt.Fprintf(&vttBuf, "X-TIMESTAMP-MAP=MPEGTS:%d,LOCAL:00:00:00.000\n\n", segmentStartPts)

				for _, req := range meta {
					var localPts int64
					if int64(req.Pts) > segmentStartPts {
						localPts = int64(req.Pts) - segmentStartPts
					}

					ms := localPts / 90
					sec := ms / 1000
					ms %= 1000
					minutes := sec / 60
					sec %= 60
					hr := minutes / 60
					minutes %= 60

					// Сделаем cue длительностью 300мс
					msEnd := ms + 300
					secEnd := sec
					if msEnd >= 1000 {
						msEnd -= 1000
						secEnd++
					}
					minEnd := minutes
					if secEnd >= 60 {
						secEnd -= 60
						minEnd++
					}
					hrEnd := hr
					if minEnd >= 60 {
						minEnd -= 60
						hrEnd++
					}

					fmt.Fprintf(&vttBuf, "%02d:%02d:%02d.%03d --> %02d:%02d:%02d.%03d\n", hr, minutes, sec, ms, hrEnd, minEnd, secEnd, msEnd)
					data, _ := json.Marshal(req)
					vttBuf.Write(data)
					vttBuf.WriteString("\n\n")
				}
				vttData = vttBuf.Bytes()
			}

			m.mu.Lock()
			m.seqCount++
			seg := &Segment{
				Name:            fmt.Sprintf("stream_%d.ts", m.seqCount),
				Duration:        frame.Timestamp - segmentStart,
				Data:            currentBuf.Bytes(),
				buf:             currentBuf,
				IsDiscontinuity: m.needsDiscontinuity,
				VTTData:         vttData,
			}
			seg.refCount.Store(1)
			m.needsDiscontinuity = false
			m.segments = append(m.segments, seg)
			m.firstSegOnce.Do(func() { close(m.firstSegReady) })
			// Оставляем в памяти только последние maxSegments сегментов
			if len(m.segments) > m.maxSegments {
				oldSeg := m.segments[0]
				m.segments = m.segments[1:]
				oldSeg.Release()
			}
			m.rebuildPlaylistsLocked()
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
			currentBuf.Reset()
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
		if writeErr := tsWriter.WriteH26x(track, pts, pts, frame.IsKeyFrame, nalus); writeErr != nil {
			// При записи в memory buffer ошибка практически невозможна
			continue
		}
	}
}

// Stop останавливает работу Muxer'а и очищает оставшиеся сегменты.
func (m *Muxer) Stop() {
	if m.unsubscribe != nil {
		m.unsubscribe()
	}
	m.cancel()
	m.firstSegOnce.Do(func() { close(m.firstSegReady) })
	m.wg.Wait()

	m.mu.Lock()
	for _, seg := range m.segments {
		seg.Release()
	}
	m.segments = nil
	m.mu.Unlock()
}

// CheckWatchdog следит за поступлением новых кадров. Если кадров нет больше 5 секунд (обрыв связи),
// он берет последний готовый сегмент и добавляет его дубликат в плейлист с флагом Discontinuity.
// Это заставляет сторонние плееры (VLC, OBS) "заморозить" последний кадр и не вылетать по таймауту.
// Метод вызывается централизованным шедулером Manager раз в секунду.
func (m *Muxer) CheckWatchdog(now time.Time) {
	lastTime := m.lastFrameTime.Load()
	if lastTime == 0 || now.Sub(time.Unix(0, lastTime)) <= 5*time.Second {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lastTime = m.lastFrameTime.Load()
	if lastTime > 0 && now.Sub(time.Unix(0, lastTime)) > 5*time.Second {
		if len(m.segments) > 0 {
			lastSeg := m.segments[len(m.segments)-1]
			m.seqCount++
			dataCopy := make([]byte, len(lastSeg.Data))
			copy(dataCopy, lastSeg.Data)
			seg := &Segment{
				Name:            fmt.Sprintf("stream_%d.ts", m.seqCount),
				Duration:        lastSeg.Duration,
				Data:            dataCopy,
				buf:             nil,
				IsDiscontinuity: true, // Сигнал для плеера, что PTS может прыгнуть
			}
			seg.refCount.Store(1)
			m.segments = append(m.segments, seg)
			m.firstSegOnce.Do(func() { close(m.firstSegReady) })
			if len(m.segments) > m.maxSegments {
				oldSeg := m.segments[0]
				m.segments = m.segments[1:]
				oldSeg.Release()
			}
			m.rebuildPlaylistsLocked()
			// Сбрасываем таймер, чтобы следующий дубликат добавился еще через 5 секунд
			m.lastFrameTime.Store(now.UnixNano())
			// Следующий РЕАЛЬНЫЙ сегмент (когда связь восстановится) тоже должен начаться с разрыва,
			// так как его PTS будет отличаться от PTS дубликата.
			m.needsDiscontinuity = true
		}
	}
}

// rebuildPlaylistsLocked генерирует строки манифестов M3U8 и атомарно обновляет их кэш.
// Вызывается ТОЛЬКО под удержанием m.mu.Lock() при смене сегментов (раз в 2 секунды).
func (m *Muxer) rebuildPlaylistsLocked() {
	if len(m.segments) == 0 {
		return
	}

	// 1. Сборка основного видео-манифеста
	pBuf := playlistPool.Get().(*[]byte)
	buf := (*pBuf)[:0]

	buf = append(buf, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:"...)

	maxDuration := 2
	for _, seg := range m.segments {
		d := int(seg.Duration.Seconds())
		if d > maxDuration {
			maxDuration = d
		}
	}
	buf = strconv.AppendInt(buf, int64(maxDuration+1), 10)
	buf = append(buf, '\n')

	seq := m.seqCount - uint64(len(m.segments)) + 1
	buf = append(buf, "#EXT-X-MEDIA-SEQUENCE:"...)
	buf = strconv.AppendUint(buf, seq, 10)
	buf = append(buf, '\n')

	for _, seg := range m.segments {
		if seg.IsDiscontinuity {
			buf = append(buf, "#EXT-X-DISCONTINUITY\n"...)
		}
		buf = append(buf, "#EXTINF:"...)
		buf = strconv.AppendFloat(buf, seg.Duration.Seconds(), 'f', 3, 64)
		buf = append(buf, ",\n"...)
		buf = append(buf, seg.Name...)
		buf = append(buf, '\n')
	}

	res := string(buf)
	if cap(buf) > 4096 {
		buf = make([]byte, 0, 1024)
	}
	*pBuf = buf
	playlistPool.Put(pBuf)
	m.cachedPlaylist.Store(&res)

	// 2. Сборка манифеста субтитров (VTT)
	pBufSubs := playlistPool.Get().(*[]byte)
	bufSubs := (*pBufSubs)[:0]

	bufSubs = append(bufSubs, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:"...)
	bufSubs = strconv.AppendInt(bufSubs, int64(maxDuration+1), 10)
	bufSubs = append(bufSubs, '\n')

	bufSubs = append(bufSubs, "#EXT-X-MEDIA-SEQUENCE:"...)
	bufSubs = strconv.AppendUint(bufSubs, seq, 10)
	bufSubs = append(bufSubs, '\n')

	for _, seg := range m.segments {
		if seg.IsDiscontinuity {
			bufSubs = append(bufSubs, "#EXT-X-DISCONTINUITY\n"...)
		}
		bufSubs = append(bufSubs, "#EXTINF:"...)
		bufSubs = strconv.AppendFloat(bufSubs, seg.Duration.Seconds(), 'f', 3, 64)
		bufSubs = append(bufSubs, ",\n"...)
		if len(seg.Name) > 3 {
			bufSubs = append(bufSubs, seg.Name[:len(seg.Name)-3]...)
		}
		bufSubs = append(bufSubs, ".vtt\n"...)
	}

	subsRes := string(bufSubs)
	if cap(bufSubs) > 4096 {
		bufSubs = make([]byte, 0, 1024)
	}
	*pBufSubs = bufSubs
	playlistPool.Put(pBufSubs)
	m.cachedSubsPlaylist.Store(&subsRes)
}

// GetPlaylist возвращает M3U8 манифест за 0 ns из атомарного кэша (Zero-Alloc, Zero-Lock).
// Принимает ctx для отслеживания отмены запроса клиентом при холодном старте.
func (m *Muxer) GetPlaylist(ctx context.Context) (string, bool) {
	// Fast-path: если кэшированный манифест уже готов (99.9% запросов)
	if p := m.cachedPlaylist.Load(); p != nil {
		return *p, true
	}

	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.RLock()
	hasSegments := len(m.segments) > 0
	m.mu.RUnlock()

	if !hasSegments {
		// Ждем первого сегмента эффективно через channel
		select {
		case <-m.firstSegReady:
		case <-m.ctx.Done():
			return "", false
		case <-ctx.Done():
			return "", false
		}
	}

	if p := m.cachedPlaylist.Load(); p != nil {
		return *p, true
	}

	// Fallback при ручном добавлении сегментов (в unit-тестах)
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.segments) == 0 {
		return "", false
	}
	m.rebuildPlaylistsLocked()
	if p := m.cachedPlaylist.Load(); p != nil {
		return *p, true
	}

	return "", false
}

// GetSubsPlaylist возвращает M3U8 манифест субтитров из атомарного кэша.
func (m *Muxer) GetSubsPlaylist() string {
	if p := m.cachedSubsPlaylist.Load(); p != nil {
		return *p
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.segments) == 0 {
		return ""
	}
	m.rebuildPlaylistsLocked()
	if p := m.cachedSubsPlaylist.Load(); p != nil {
		return *p
	}
	return ""
}

// AcquireSegment возвращает сегмент без копирования памяти с атомарным захватом ссылки (Zero-Alloc, Zero-Copy).
func (m *Muxer) AcquireSegment(name string) (*Segment, string) {
	isVTT := len(name) > 4 && name[len(name)-4:] == ".vtt"
	tsName := name
	if isVTT {
		tsName = name[:len(name)-4] + ".ts"
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, seg := range m.segments {
		if seg.Name == tsName {
			seg.Retain()
			if isVTT {
				return seg, "text/vtt"
			}
			return seg, "video/mp2t"
		}
	}
	return nil, ""
}

// GetSegment возвращает данные TS или VTT сегмента (для обратной совместимости).
func (m *Muxer) GetSegment(name string) ([]byte, string) {
	seg, mimeType := m.AcquireSegment(name)
	if seg == nil {
		return nil, ""
	}
	defer seg.Release()
	if mimeType == "text/vtt" {
		resp := make([]byte, len(seg.VTTData))
		copy(resp, seg.VTTData)
		return resp, mimeType
	}
	resp := make([]byte, len(seg.Data))
	copy(resp, seg.Data)
	return resp, mimeType
}
