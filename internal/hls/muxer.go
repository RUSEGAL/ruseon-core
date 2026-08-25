// Package hls provides real-time HTTP Live Streaming (HLS RFC 8216) packaging from RingBuffer video frames,
// sliding window M3U8 playlist generation, zero-copy ARC memory segment recycling, and WebVTT metadata streaming.
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

	"github.com/RUSEGAL/ruseon-core/v2/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/v2/internal/rtsp"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/grpc/pb"
)

var emptyVTTData = []byte("WEBVTT\n\n")

// Segment represents a single in-memory MPEG-TS media segment and accompanying WebVTT subtitle cue data.
//
// Memory optimization:
// Uses Atomic Reference Counting (ARC). The underlying byte buffer is allocated from a sync.Pool
// and automatically returned to the pool when all concurrent HTTP read handlers invoke Release().
type Segment struct {
	// Name is the segment filename (e.g. "segment_123.ts").
	Name string
	// Duration is the calculated playback duration of the segment.
	Duration time.Duration
	// Data is the raw MPEG-TS container binary payload.
	Data []byte
	buf *bytes.Buffer
	refCount atomic.Int32
	// IsDiscontinuity indicates whether an #EXT-X-DISCONTINUITY tag precedes this segment.
	IsDiscontinuity bool
	// VTTData holds the WebVTT subtitle track cues for AI bounding boxes and detections.
	VTTData []byte
}

// Retain increments the atomic reference counter of the segment.
func (s *Segment) Retain() {
	s.refCount.Add(1)
}

// Release decrements the atomic reference counter.
// When the counter drops to zero, the underlying buffer is reset and returned to sync.Pool.
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

// Muxer consumes video frames from a RingBuffer, muxes them into MPEG-TS chunks,
// maintains a sliding window playlist, and serves cached M3U8 manifests.
//
// Concurrency & Zero-Copy guarantees:
//   - Manifest generation is lock-free and zero-alloc via atomic.Pointer caching.
//   - Segments are retrieved with O(1) hash map indexing and ARC memory protection.
//   - Inactivity and freeze watchdogs ensure stream continuity across RTSP dropouts.
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
	segIndex map[string]*Segment
	seqCount uint64

	currentMeta []*pb.MetadataRequest

	lastFrameTime      atomic.Int64
	needsDiscontinuity bool

	firstSegReady chan struct{}
	firstSegOnce  sync.Once

	cachedPlaylist     atomic.Pointer[string]
	cachedSubsPlaylist atomic.Pointer[string]
}

// NewMuxer constructs and launches a new HLS Muxer with the specified sliding window segment limit.
// If maxSegments is omitted, a default sliding window of 3 live segments is maintained (RFC 8216).
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
		segIndex:       make(map[string]*Segment, maxSegs+2),
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
	params := m.ringBuffer.GetCodecParams()
	var vps, sps, pps []byte
	if params != nil {
		vps, sps, pps = params.VPS, params.SPS, params.PPS
	}

	metaChan := m.metaChan
	mNalus := make([][]byte, 0, 16)
	var frameCount int

	for {
		if sps == nil || pps == nil {
			if p := m.ringBuffer.GetCodecParams(); p != nil {
				vps, sps, pps = p.VPS, p.SPS, p.PPS
			}
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

		// Обновляем watchdog раз в 25 кадров (~1 сек) или на ключевых кадрах, сокращая вызовы time.Now() на 96%
		frameCount++
		if frame.IsKeyFrame || frameCount >= 25 {
			frameCount = 0
			m.lastFrameTime.Store(time.Now().UnixNano())
		}

		if frame.IsKeyFrame {
			newParams := m.ringBuffer.GetCodecParams()
			if newParams != nil && newParams.SPS != nil {
				if !bytes.Equal(sps, newParams.SPS) || !bytes.Equal(pps, newParams.PPS) || !bytes.Equal(vps, newParams.VPS) {
					vps, sps, pps = newParams.VPS, newParams.SPS, newParams.PPS
					m.needsDiscontinuity = true
				}
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
				segmentStartPts := rtsp.DurationTo90k(segmentStart)
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
				mNalus = mNalus[:0]
				if vps != nil {
					mNalus = append(mNalus, vps, sps, pps)
				} else {
					mNalus = append(mNalus, sps, pps)
				}
				mNalus = append(mNalus, nalus...)
				nalus = mNalus
			}
		}

		// Переводим duration в 90kHz тики (стандарт для MPEG-TS)
		pts := rtsp.DurationTo90k(frame.Timestamp)

		// Пишем NALU в TS
		// Так как мы не имеем B-кадров от большинства IP камер, PTS == DTS.
		if writeErr := tsWriter.WriteH26x(track, pts, pts, frame.IsKeyFrame, nalus); writeErr != nil {
			// При записи в memory buffer ошибка практически невозможна
			continue
		}
	}
}

// Stop terminates the HLS muxing loop, cancels background readers, and releases all active segments back to sync.Pool.
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
	m.segIndex = nil
	m.mu.Unlock()
}

// CheckWatchdog monitors stream arrival times. If no frames arrive for >5 seconds (e.g. RTSP dropout),
// it duplicates the last segment in the playlist with an #EXT-X-DISCONTINUITY tag.
// This prevents external players (VLC, OBS, browsers) from stalling or disconnecting during temporary interruptions.
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
				IsDiscontinuity: true, // Signal to player that PTS discontinuity occurred
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
			// Reset watchdog timer for next 5-second interval
			m.lastFrameTime.Store(now.UnixNano())
			// Flag next real segment with discontinuity as PTS will jump
			m.needsDiscontinuity = true
		}
	}
}

// rebuildPlaylistsLocked renders M3U8 playlist strings and updates atomic cached pointers.
// Must be invoked exclusively under m.mu.Lock().
func (m *Muxer) rebuildPlaylistsLocked() {
	if len(m.segments) == 0 {
		m.segIndex = make(map[string]*Segment)
		return
	}

	// Update O(1) hash index for fast segment lookups
	m.segIndex = make(map[string]*Segment, len(m.segments))
	for _, seg := range m.segments {
		m.segIndex[seg.Name] = seg
	}

	// 1. Build video M3U8 manifest
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

	// 2. Build WebVTT subtitle M3U8 manifest
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

// GetPlaylist retrieves the active M3U8 manifest string from the atomic cache (lock-free, zero-allocation).
//
// If invoked on a freshly started muxer where no segments have completed yet, it awaits the first
// completed segment or context cancellation.
func (m *Muxer) GetPlaylist(ctx context.Context) (string, bool) {
	// Fast-path: pre-rendered manifest from atomic pointer cache
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
		// Await first segment completion via channel
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

	// Fallback for tests
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

// GetSubsPlaylist retrieves the cached WebVTT subtitle M3U8 manifest string.
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

// AcquireSegment looks up a media segment or WebVTT cue by filename in O(1) time
// and increments its ARC reference count to guarantee safe concurrent reads without memory copying.
//
// The caller MUST call seg.Release() after serving the HTTP payload to return the buffer to the pool.
func (m *Muxer) AcquireSegment(name string) (*Segment, string) {
	isVTT := len(name) > 4 && name[len(name)-4:] == ".vtt"
	tsName := name
	if isVTT {
		tsName = name[:len(name)-4] + ".ts"
	}

	m.mu.RLock()
	seg := m.segIndex[tsName]
	if seg == nil {
		for _, s := range m.segments {
			if s.Name == tsName {
				seg = s
				break
			}
		}
	}
	if seg != nil {
		seg.Retain()
	}
	m.mu.RUnlock()

	if seg == nil {
		return nil, ""
	}
	if isVTT {
		return seg, "text/vtt"
	}
	return seg, "video/mp2t"
}

// GetSegment copies the segment payload into a newly allocated heap byte slice.
//
// Deprecated: Use AcquireSegment instead for zero-copy memory access with ARC reference counting.
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
