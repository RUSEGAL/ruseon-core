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

type PartInfo struct {
	Offset   int64
	Duration uint32 // In 90kHz ticks
}

type FileIndex struct {
	InitOffset int64
	InitSize   int64
	Parts      []PartInfo
}

type lruEntry struct {
	key   string
	value *FileIndex
}

// IndexLRUCache реализует потокобезопасный LRU-кэш для индексов fMP4 файлов.
type IndexLRUCache struct {
	mu        sync.Mutex
	capacity  int
	items     map[string]*list.Element
	evictList *list.List
}

// NewIndexLRUCache создает новый LRU-кэш индексов заданной емкости.
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

// Get возвращает индекс файла из кэша.
func (c *IndexLRUCache) Get(key string) (*FileIndex, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		return elem.Value.(*lruEntry).value, true
	}
	return nil, false
}

// Add сохраняет индекс файла в кэш с вытеснением наименее используемых элементов.
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

// Remove удаляет запись из кэша.
func (c *IndexLRUCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		delete(c.items, key)
		c.evictList.Remove(elem)
	}
}

// Clear очищает весь кэш.
func (c *IndexLRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.evictList.Init()
}

// Len возвращает текущее количество элементов в кэше.
func (c *IndexLRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictList.Len()
}

var globalIndexCache = NewIndexLRUCache(defaultCacheCapacity)

// InvalidateFileIndex удаляет файл из глобального кэша индексов (например, при удалении файла).
func InvalidateFileIndex(path string) {
	globalIndexCache.Remove(path)
}

// ClearIndexCache полностью очищает глобальный кэш индексов.
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

// GenerateHLSPlaylist генерирует M3U8 манифест для конкретного MP4 файла
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

// GenerateHLSSegment читает fMP4 фрагмент и перепаковывает его в MPEG-TS "на лету"
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

	// 3. Создаем MPEG-TS Writer в памяти
	outBuf := bytes.NewBuffer(make([]byte, 0, 1024*1024)) // Этап 23.3: Preallocate to avoid GC pressure
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

	return outBuf.Bytes(), nil
}
