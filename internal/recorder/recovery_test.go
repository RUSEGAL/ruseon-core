package recorder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
)

func createValidFMP4File(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	init := &fmp4.Init{
		Tracks: []*fmp4.InitTrack{{
			ID:        1,
			TimeScale: 90000,
			Codec:     &fmp4.CodecH264{SPS: sps, PPS: pps},
		}},
	}
	if err := init.Marshal(file); err != nil {
		return err
	}

	sample, _ := fmp4.NewPartSampleH26x(0, true, [][]byte{{0x05, 0x01, 0x02, 0x03}})
	sample.Duration = 90000 / 25
	part := &fmp4.Part{
		SequenceNumber: 1,
		Tracks: []*fmp4.PartTrack{{
			ID:       1,
			BaseTime: 0,
			Samples:  []*fmp4.PartSample{sample},
		}},
	}
	return part.Marshal(file)
}

func TestValidateFMP4File(t *testing.T) {
	tempDir := t.TempDir()

	// 1. 0-byte file -> invalid
	emptyPath := filepath.Join(tempDir, "empty.mp4")
	fEmpty, _ := os.Create(emptyPath)
	fEmpty.Close()
	if valid, _ := ValidateFMP4File(emptyPath); valid {
		t.Errorf("expected 0-byte file to be invalid")
	}

	// 2. Corrupted / random bytes -> invalid
	junkPath := filepath.Join(tempDir, "junk.mp4")
	_ = os.WriteFile(junkPath, []byte("random non-fmp4 garbage byte stream"), 0600)
	if valid, _ := ValidateFMP4File(junkPath); valid {
		t.Errorf("expected junk file to be invalid")
	}

	// 3. Init only (no media fragments) -> invalid
	initOnlyPath := filepath.Join(tempDir, "init_only.mp4")
	fInit, _ := os.Create(initOnlyPath)
	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	init := &fmp4.Init{
		Tracks: []*fmp4.InitTrack{{
			ID:        1,
			TimeScale: 90000,
			Codec:     &fmp4.CodecH264{SPS: sps, PPS: pps},
		}},
	}
	_ = init.Marshal(fInit)
	fInit.Close()
	if valid, _ := ValidateFMP4File(initOnlyPath); valid {
		t.Errorf("expected init-only file without media parts to be invalid")
	}

	// 4. Valid fMP4 file -> valid
	validPath := filepath.Join(tempDir, "valid.mp4")
	if err := createValidFMP4File(validPath); err != nil {
		t.Fatalf("failed to create valid fMP4: %v", err)
	}
	if valid, err := ValidateFMP4File(validPath); !valid || err != nil {
		t.Errorf("expected valid fMP4 to pass validation, got valid=%v, err=%v", valid, err)
	}
}

func TestRecoverCrashedFiles(t *testing.T) {
	tempDir := t.TempDir()
	camDir := filepath.Join(tempDir, "cam1")
	
	err := os.MkdirAll(camDir, 0755)
	if err != nil {
		t.Fatalf("failed to create cam dir: %v", err)
	}

	// 1. Создаем валидный упавший файл
	validCrashedPath := filepath.Join(camDir, "2026-07-31_15-04-05_ongoing.mp4")
	if err := createValidFMP4File(validCrashedPath); err != nil {
		t.Fatalf("failed to create valid fMP4: %v", err)
	}
	crashTime := time.Date(2026, 7, 31, 15, 10, 30, 0, time.Local)
	_ = os.Chtimes(validCrashedPath, crashTime, crashTime)

	// 2. Создаем битый/пустой 0-байтовый упавший файл
	corruptedCrashedPath := filepath.Join(camDir, "2026-07-31_16-00-00_ongoing.mp4")
	fEmpty, _ := os.Create(corruptedCrashedPath)
	fEmpty.Close()

	// 3. Создаем нормальный завершенный файл архива
	normalPath := filepath.Join(camDir, "2026-07-31_12-00-00_to_13-00-00.mp4")
	f2, _ := os.Create(normalPath)
	f2.Close()

	// Запускаем восстановление
	RecoverCrashedFiles(tempDir)

	// Проверяем: валидный упавший файл переименован в нормальный архив
	if _, err := os.Stat(validCrashedPath); !os.IsNotExist(err) {
		t.Errorf("valid crashed file should have been renamed")
	}
	expectedRecoveredPath := filepath.Join(camDir, "2026-07-31_15-04-05_to_15-10-30.mp4")
	if _, err := os.Stat(expectedRecoveredPath); os.IsNotExist(err) {
		t.Errorf("expected recovered file to exist at %s", expectedRecoveredPath)
	}

	// Проверяем: битый файл изолирован с суффиксом .corrupted и не попал в архив
	if _, err := os.Stat(corruptedCrashedPath); !os.IsNotExist(err) {
		t.Errorf("corrupted ongoing file should have been removed or renamed")
	}
	corruptedTarget := filepath.Join(camDir, "2026-07-31_16-00-00_ongoing.corrupted")
	if _, err := os.Stat(corruptedTarget); os.IsNotExist(err) {
		t.Errorf("expected corrupted file to be isolated at %s", corruptedTarget)
	}

	// Проверяем: нормальный файл остался нетронутым
	if _, err := os.Stat(normalPath); os.IsNotExist(err) {
		t.Errorf("normal file should not have been touched")
	}
}

func TestValidateFMP4File_TruncatedBoxPayload(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "truncated_box.mp4")

	// 1. Создаем валидный fMP4
	if err := createValidFMP4File(filePath); err != nil {
		t.Fatalf("failed to create valid fMP4: %v", err)
	}

	// 2. Дописываем заголовок mdat, заявляющий 1 МБ (1048576 байт), но с телом всего в 16 байт
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	// 4 байта size (0x00100000 = 1048576) + 4 байта type "mdat"
	var fakeHeader [8]byte
	fakeHeader[0] = 0x00
	fakeHeader[1] = 0x10
	fakeHeader[2] = 0x00
	fakeHeader[3] = 0x00
	copy(fakeHeader[4:8], "mdat")
	_, _ = file.Write(fakeHeader[:])
	_, _ = file.Write(make([]byte, 16)) // пишем только 16 байт вместо 1048568
	file.Close()

	// 3. Валидатор должен обнаружить превышение размера над физической длиной файла
	valid, err := ValidateFMP4File(filePath)
	if valid || err == nil {
		t.Errorf("expected ValidateFMP4File to fail for truncated box payload, got valid=%v, err=%v", valid, err)
	}
}

func TestRecoverCrashedFiles_TruncatedTrailingFragment(t *testing.T) {
	tempDir := t.TempDir()
	camDir := filepath.Join(tempDir, "cam_truncated")
	_ = os.MkdirAll(camDir, 0755)

	truncatedOngoingPath := filepath.Join(camDir, "2026-07-31_17-00-00_ongoing.mp4")
	if err := createValidFMP4File(truncatedOngoingPath); err != nil {
		t.Fatalf("failed to create valid fMP4: %v", err)
	}

	// Дописываем обрезанный moof/mdat
	file, _ := os.OpenFile(truncatedOngoingPath, os.O_WRONLY|os.O_APPEND, 0600)
	var fakeHeader [8]byte
	fakeHeader[0] = 0x00
	fakeHeader[1] = 0x05
	fakeHeader[2] = 0x00
	fakeHeader[3] = 0x00
	copy(fakeHeader[4:8], "moof")
	_, _ = file.Write(fakeHeader[:])
	_, _ = file.Write(make([]byte, 10))
	file.Close()

	// Запускаем восстановление
	RecoverCrashedFiles(tempDir)

	// Файл должен быть изолирован в .corrupted, а не переименован в .mp4
	corruptedTarget := filepath.Join(camDir, "2026-07-31_17-00-00_ongoing.corrupted")
	if _, err := os.Stat(corruptedTarget); os.IsNotExist(err) {
		t.Errorf("expected truncated ongoing recording to be isolated as .corrupted, not found at %s", corruptedTarget)
	}
}
