package archive

import (
	"bytes"
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

type PartInfo struct {
	Offset   int64
	Duration uint32 // In 90kHz ticks
}

type FileIndex struct {
	InitOffset int64
	InitSize   int64
	Parts      []PartInfo
}

var (
	indexCache = make(map[string]*FileIndex)
	cacheMu    sync.RWMutex
)

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
	cacheMu.RLock()
	idx, ok := indexCache[path]
	cacheMu.RUnlock()
	if ok {
		return idx, nil
	}

	log.Info().Str("file", path).Msg("Indexing archive file for HLS")

	f, err := registry.CurrentBlobStore.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	idx = &FileIndex{}
	
	// Находим размер Init блока (ftyp + moov) вручную, 
	// так как fmp4.Init.Unmarshal сканирует весь файл до конца
	var initSize int64
	for {
		typ, size, err := readBoxHeader(f)
		if err != nil {
			break
		}
		if typ == "moof" {
			// Нашли первый moof, возвращаемся к его началу
			_, _ = f.Seek(-8, io.SeekCurrent)
			break
		}
		initSize += int64(size)
		_, _ = f.Seek(int64(size)-8, io.SeekCurrent)
	}

	idx.InitOffset = 0
	idx.InitSize = initSize
	_, _ = f.Seek(initSize, io.SeekStart)

	for {
		offset, _ := f.Seek(0, io.SeekCurrent)
		typ, size, err := readBoxHeader(f)
		if err != nil {
			break
		}
		
		if typ == "moof" {
			// Нашли moof, читаем его целиком
			_, _ = f.Seek(offset, io.SeekStart)
			moofData := make([]byte, size)
			_, _ = io.ReadFull(f, moofData)
			
			// Дальше должен быть mdat
			mdatOffset, _ := f.Seek(0, io.SeekCurrent)
			typ2, size2, err2 := readBoxHeader(f)
			if err2 == nil && typ2 == "mdat" {
				_, _ = f.Seek(mdatOffset, io.SeekStart)
				mdatData := make([]byte, size2)
				_, _ = io.ReadFull(f, mdatData)
				
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
			_, _ = f.Seek(offset+int64(size), io.SeekStart)
		}
	}

	cacheMu.Lock()
	indexCache[path] = idx
	cacheMu.Unlock()

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
	_, _ = f.Seek(idx.InitOffset, io.SeekStart)
	var init fmp4.Init
	if err := init.Unmarshal(f); err != nil {
		return nil, err
	}
	
	if len(init.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks in init")
	}
	
	// 2. Читаем запрашиваемый Part
	_, _ = f.Seek(idx.Parts[seq].Offset, io.SeekStart)
	
	// Читаем moof
	_, moofSize, _ := readBoxHeader(f)
	_, _ = f.Seek(idx.Parts[seq].Offset, io.SeekStart)
	moofData := make([]byte, moofSize)
	_, _ = io.ReadFull(f, moofData)
	
	// Читаем mdat
	_, mdatSize, _ := readBoxHeader(f)
	_, _ = f.Seek(idx.Parts[seq].Offset+int64(moofSize), io.SeekStart)
	mdatData := make([]byte, mdatSize)
	_, _ = io.ReadFull(f, mdatData)
	
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
