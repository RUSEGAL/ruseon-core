package archive

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/RUSEGAL/ruseon-core/pkg/registry"

	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
	"github.com/rs/zerolog/log"
)

const (
	defaultCacheCapacity = 2000
	minBoxHeaderSize     = 8
	maxBoxSize           = 64 * 1024 * 1024 // 64 MB максимальный размер бокса для защиты от OOM
)

var tsBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024*1024))
	},
}

// PartInfo records the physical file byte offset and presentation duration of an fMP4 fragment (`moof` + `mdat`).
type PartInfo struct {
	// Offset is the byte position of the fragment's `moof` box in the MP4 file.
	Offset int64
	// Duration is the cumulative sample duration in 90kHz RTP timescale ticks.
	Duration uint32
}

// FileIndex stores the movie initialization box offsets and media fragment table for an fMP4 recording.
type FileIndex struct {
	// InitOffset is the starting byte offset of the `ftyp` + `moov` initialization header.
	InitOffset int64
	// InitSize is the total byte size of the initialization header.
	InitSize int64
	// Parts contains indexed metadata for each sequential media fragment in the file.
	Parts []PartInfo
}

type lruEntry struct {
	key   string
	value *FileIndex
}

// IndexLRUCache provides a thread-safe LRU cache of parsed FileIndex structures to avoid repeated disk header parsing.
type IndexLRUCache struct {
	mu        sync.Mutex
	capacity  int
	items     map[string]*list.Element
	evictList *list.List
}

// NewIndexLRUCache creates an IndexLRUCache with the specified item capacity.
func NewIndexLRUCache(capacity int) *IndexLRUCache {
	if capacity <= 0 {
		capacity = defaultCacheCapacity
	}
	return &IndexLRUCache{
		capacity:  capacity,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a FileIndex by file path, updating its LRU position.
func (c *IndexLRUCache) Get(key string) (*FileIndex, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		return elem.Value.(*lruEntry).value, true
	}
	return nil, false
}

// Add stores a FileIndex in the LRU cache, evicting the least recently used element when capacity is exceeded.
func (c *IndexLRUCache) Add(key string, value *FileIndex) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		elem.Value.(*lruEntry).value = value
		return
	}

	for c.evictList.Len() >= c.capacity {
		oldest := c.evictList.Back()
		if oldest != nil {
			delete(c.items, oldest.Value.(*lruEntry).key)
			c.evictList.Remove(oldest)
		}
	}

	entry := &lruEntry{key: key, value: value}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem
}

// Remove deletes a file index from the cache.
func (c *IndexLRUCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		delete(c.items, key)
		c.evictList.Remove(elem)
	}
}

// Clear flushes all cached file indices.
func (c *IndexLRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.evictList.Init()
}

// Len returns the current number of cached file indices.
func (c *IndexLRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictList.Len()
}

var globalIndexCache = NewIndexLRUCache(defaultCacheCapacity)

// InvalidateFileIndex removes an entry from the global file index cache (e.g. when an MP4 file is deleted).
func InvalidateFileIndex(path string) {
	globalIndexCache.Remove(path)
}

// ClearIndexCache clears all entries from the global file index cache.
func ClearIndexCache() {
	globalIndexCache.Clear()
}

func readBoxHeader(r io.Reader) (string, uint32, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return "", 0, err
	}
	size := binary.BigEndian.Uint32(header[0:4])
	boxType := string(header[4:8])
	return boxType, size, nil
}

// getFileIndex сканирует fMP4 файл и запоминает смещения каждого Part (фрагмента)
func getFileIndex(path string) (*FileIndex, error) {
	if idx, ok := globalIndexCache.Get(path); ok {
		return idx, nil
	}

	log.Info().Str("file", path).Msg("Indexing archive file for HLS")

	f, err := registry.CurrentBlobStore.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	idx := &FileIndex{}
	
	// Находим размер Init блока (ftyp + moov) вручную, 
	// так как fmp4.Init.Unmarshal сканирует весь файл до конца
	var initSize int64
	for {
		typ, size, err := readBoxHeader(f)
		if err != nil {
			break
		}
		if size < minBoxHeaderSize || size > maxBoxSize {
			log.Warn().Str("file", path).Str("type", typ).Uint32("size", size).Msg("Invalid or excessive box size in init section")
			break
		}
		if typ == "moof" {
			// Нашли первый moof, возвращаемся к его началу
			if _, err := f.Seek(-minBoxHeaderSize, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("failed to seek before moof: %w", err)
			}
			break
		}
		initSize += int64(size)
		if _, err := f.Seek(int64(size)-minBoxHeaderSize, io.SeekCurrent); err != nil {
			break
		}
	}

	idx.InitOffset = 0
	idx.InitSize = initSize
	if _, err := f.Seek(initSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to initSize %d: %w", initSize, err)
	}

	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			break
		}
		typ, size, err := readBoxHeader(f)
		if err != nil {
			break
		}
		if size < minBoxHeaderSize || size > maxBoxSize {
			log.Warn().Str("file", path).Str("type", typ).Uint32("size", size).Msg("Invalid or excessive box size in parts section")
			break
		}
		
		if typ == "moof" {
			// Нашли moof, читаем его целиком
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				break
			}
			moofData := make([]byte, size)
			if _, err := io.ReadFull(f, moofData); err != nil {
				break
			}
			
			// Дальше должен быть mdat
			mdatOffset, err := f.Seek(0, io.SeekCurrent)
			if err != nil {
				break
			}
			typ2, size2, err2 := readBoxHeader(f)
			if err2 == nil && typ2 == "mdat" && size2 >= minBoxHeaderSize && size2 <= maxBoxSize {
				if _, err := f.Seek(mdatOffset, io.SeekStart); err != nil {
					break
				}
				mdatData := make([]byte, size2)
				if _, err := io.ReadFull(f, mdatData); err != nil {
					break
				}
				
				// Парсим Part
				combined := make([]byte, 0, len(moofData)+len(mdatData))
				combined = append(combined, moofData...)
				combined = append(combined, mdatData...)
				
				var parts fmp4.Parts
				var dur uint32 = 90000 // Fallback: 1 second
				if err := parts.Unmarshal(combined); err == nil && len(parts) > 0 {
					part := parts[0]
					var parsedDur uint32
					if len(part.Tracks) > 0 {
						for _, s := range part.Tracks[0].Samples {
							parsedDur += s.Duration
						}
					}
					if parsedDur > 0 {
						dur = parsedDur
					}
				}
				
				idx.Parts = append(idx.Parts, PartInfo{
					Offset:   offset,
					Duration: dur,
				})
			}
		} else {
			// Пропускаем неизвестный бокс
			if _, err := f.Seek(offset+int64(size), io.SeekStart); err != nil {
				break
			}
		}
	}

	globalIndexCache.Add(path, idx)

	return idx, nil
}

// GenerateHLSPlaylist compiles a VOD M3U8 manifest representing each fMP4 fragment as an individual HLS TS segment.
func GenerateHLSPlaylist(recordDir, cameraID, filename string) (string, error) {
	path := filepath.Join(recordDir, cameraID, filename)
	idx, err := getFileIndex(path)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString("#EXTM3U\n")
	buf.WriteString("#EXT-X-VERSION:3\n")
	buf.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	
	maxDurSec := 2
	for _, p := range idx.Parts {
		dur := int((p.Duration / 90000) + 1)
		if dur > maxDurSec {
			maxDurSec = dur
		}
	}
	fmt.Fprintf(&buf, "#EXT-X-TARGETDURATION:%d\n", maxDurSec)
	buf.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")

	for i, p := range idx.Parts {
		durSec := float64(p.Duration) / 90000.0
		fmt.Fprintf(&buf, "#EXTINF:%.3f,\n", durSec)
		fmt.Fprintf(&buf, "segment.ts?file=%s&seq=%d\n", filename, i)
	}
	
	buf.WriteString("#EXT-X-ENDLIST\n")

	return buf.String(), nil
}

// GenerateHLSSegment extracts an individual fMP4 media fragment and transcodes it on-the-fly into an MPEG-TS segment byte slice.
func GenerateHLSSegment(recordDir, cameraID, filename string, seq int) ([]byte, error) {
	path := filepath.Join(recordDir, cameraID, filename)
	idx, err := getFileIndex(path)
	if err != nil {
		return nil, err
	}

	if seq < 0 || seq >= len(idx.Parts) {
		return nil, fmt.Errorf("segment out of bounds")
	}

	f, err := registry.CurrentBlobStore.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 1. Читаем Init чтобы достать параметры кодека (SPS/PPS)
	if _, err := f.Seek(idx.InitOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek init: %w", err)
	}
	var init fmp4.Init
	if err := init.Unmarshal(f); err != nil {
		return nil, err
	}
	
	if len(init.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks in init")
	}
	
	// 2. Читаем запрашиваемый Part
	if _, err := f.Seek(idx.Parts[seq].Offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek part: %w", err)
	}
	
	// Читаем moof
	_, moofSize, err := readBoxHeader(f)
	if err != nil || moofSize < minBoxHeaderSize || moofSize > maxBoxSize {
		return nil, fmt.Errorf("invalid moof header: %w", err)
	}
	if _, err := f.Seek(idx.Parts[seq].Offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek moof: %w", err)
	}
	moofData := make([]byte, moofSize)
	if _, err := io.ReadFull(f, moofData); err != nil {
		return nil, fmt.Errorf("failed to read moof: %w", err)
	}
	
	// Читаем mdat
	_, mdatSize, err := readBoxHeader(f)
	if err != nil || mdatSize < minBoxHeaderSize || mdatSize > maxBoxSize {
		return nil, fmt.Errorf("invalid mdat header: %w", err)
	}
	if _, err := f.Seek(idx.Parts[seq].Offset+int64(moofSize), io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek mdat: %w", err)
	}
	mdatData := make([]byte, mdatSize)
	if _, err := io.ReadFull(f, mdatData); err != nil {
		return nil, fmt.Errorf("failed to read mdat: %w", err)
	}
	
	combined := make([]byte, 0, len(moofData)+len(mdatData))
	combined = append(combined, moofData...)
	combined = append(combined, mdatData...)
	
	var parts fmp4.Parts
	if err := parts.Unmarshal(combined); err != nil || len(parts) == 0 {
		return nil, fmt.Errorf("failed to unmarshal part")
	}
	part := parts[0]
	
	if len(part.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks in part")
	}

	// 3. Создаем MPEG-TS Writer в памяти с переиспользованием буфера из пула
	outBuf := tsBufferPool.Get().(*bytes.Buffer)
	outBuf.Reset()
	defer func() {
		outBuf.Reset()
		tsBufferPool.Put(outBuf)
	}()

	var tsTrack *mpegts.Track
	var tsWriter *mpegts.Writer

	initTrack := init.Tracks[0]
	switch initTrack.Codec.(type) {
	case *fmp4.CodecH264:
		tsTrack = &mpegts.Track{Codec: &mpegts.CodecH264{}}
	case *fmp4.CodecH265:
		tsTrack = &mpegts.Track{Codec: &mpegts.CodecH265{}}
	default:
		return nil, fmt.Errorf("unsupported codec")
	}

	tsWriter = mpegts.NewWriter(outBuf, []*mpegts.Track{tsTrack})

	// 4. Перепаковываем NALU в TS
	pts := int64(part.Tracks[0].BaseTime) // #nosec G115
	for _, sample := range part.Tracks[0].Samples {
		isKeyFrame := !sample.IsNonSyncSample
		
		var nalus [][]byte
		var err error
		
		switch c := initTrack.Codec.(type) {
		case *fmp4.CodecH264:
			nalus, err = sample.GetH26x()
			if isKeyFrame {
				nalus = append([][]byte{c.SPS, c.PPS}, nalus...)
			}
		case *fmp4.CodecH265:
			nalus, err = sample.GetH26x()
			if isKeyFrame {
				nalus = append([][]byte{c.VPS, c.SPS, c.PPS}, nalus...)
			}
		}
		
		if err != nil {
			continue
		}

		err = tsWriter.WriteH26x(tsTrack, pts, pts, isKeyFrame, nalus)
		if err != nil {
			log.Error().Err(err).Msg("Failed to mux NALU to TS")
		}

		pts += int64(sample.Duration)
	}

	res := make([]byte, outBuf.Len())
	copy(res, outBuf.Bytes())
	return res, nil
}
