package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExportMP4 читает нужные фрагменты из fMP4 файла и напрямую стримит их в `io.Writer` (например, HTTP-ответ)
func ExportMP4(recordDir, cameraID, filename string, startSeq, endSeq int, w io.Writer) error {
	path := filepath.Join(recordDir, cameraID, filename)
	idx, err := getFileIndex(path)
	if err != nil {
		return err
	}

	if startSeq < 0 {
		startSeq = 0
	}
	if endSeq >= len(idx.Parts) || endSeq < 0 {
		endSeq = len(idx.Parts) - 1
	}
	if startSeq > endSeq {
		startSeq = endSeq
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// 1. Копируем Init блок (ftyp + moov)
	f.Seek(idx.InitOffset, io.SeekStart)
	_, err = io.CopyN(w, f, idx.InitSize)
	if err != nil {
		return fmt.Errorf("failed to write init block: %w", err)
	}

	// 2. Копируем выбранные Part блоки (moof + mdat)
	for i := startSeq; i <= endSeq; i++ {
		startOffset := idx.Parts[i].Offset
		
		var partSize int64
		if i < len(idx.Parts)-1 {
			// Размер части = начало следующей - начало текущей
			partSize = idx.Parts[i+1].Offset - startOffset
		} else {
			// Для последней части читаем до конца файла
			info, err := f.Stat()
			if err != nil {
				return err
			}
			partSize = info.Size() - startOffset
		}

		f.Seek(startOffset, io.SeekStart)
		_, err = io.CopyN(w, f, partSize)
		if err != nil {
			return fmt.Errorf("failed to write part %d: %w", i, err)
		}
	}

	return nil
}
