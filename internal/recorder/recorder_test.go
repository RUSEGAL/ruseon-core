package recorder

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/v2/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
)

func TestRecorder_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	rb := buffer.NewRingBuffer(10)
	
	// Set dummy SPS/PPS to satisfy recorder wait
	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps, pps)

	r := NewRecorder("cam1", rb, tempDir, nil)
	if r == nil {
		t.Fatalf("expected non-nil recorder")
	}

	// Wait a bit to ensure the run goroutine starts waiting on frames
	time.Sleep(100 * time.Millisecond)

	// Write a keyframe to trigger recording start
	rb.Write(&buffer.Frame{
		Timestamp:  time.Duration(90000), // 1 second
		IsKeyFrame: true,
		NALUs:      [][]byte{{0x05, 0x01, 0x02, 0x03}}, // Dummy IDR
	})

	// Write a normal frame
	time.Sleep(50 * time.Millisecond)
	rb.Write(&buffer.Frame{
		Timestamp:  time.Duration(180000), // 2 seconds
		IsKeyFrame: false,
		NALUs:      [][]byte{{0x01, 0x01, 0x02, 0x03}}, // Dummy Non-IDR
	})

	// Wait for recorder to process the frames
	time.Sleep(200 * time.Millisecond)

	// Stop recorder
	r.Stop()
	rb.Close() // Wake up the reader
	time.Sleep(200 * time.Millisecond)

	// Check if directory cam1 exists
	camDir := filepath.Join(tempDir, "cam1")
	if _, err := os.Stat(camDir); os.IsNotExist(err) {
		t.Fatalf("expected directory %s to be created", camDir)
	}

	// Check if file was created and renamed properly
	entries, err := os.ReadDir(camDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatalf("expected mp4 files in directory, got none")
	}

	foundFinal := false
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".mp4") && !strings.HasSuffix(name, "_ongoing.mp4") {
			foundFinal = true
		}
	}

	if !foundFinal {
		t.Errorf("expected a finalized .mp4 file (renamed from _ongoing), found none. Files: %v", entries)
	}
}

func TestRecorder_EmptySegmentCleaned(t *testing.T) {
	tempDir := t.TempDir()
	rb := buffer.NewRingBuffer(10)
	
	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps, pps)

	r := NewRecorder("cam_empty", rb, tempDir, nil)
	time.Sleep(50 * time.Millisecond)

	// Не пишем никаких кадров, сразу останавливаем рекордер
	r.Stop()
	rb.Close()
	time.Sleep(50 * time.Millisecond)

	camDir := filepath.Join(tempDir, "cam_empty")
	if entries, err := os.ReadDir(camDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".mp4") {
				t.Errorf("expected no mp4 files for empty recorder, found: %s", entry.Name())
			}
		}
	}
}

func TestRecorder_TimestampRegression_And_BFrames(t *testing.T) {
	tempDir := t.TempDir()
	rb := buffer.NewRingBuffer(10)
	
	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps, pps)

	r := NewRecorder("cam_bframe", rb, tempDir, nil)
	time.Sleep(50 * time.Millisecond)

	// 1. Keyframe at PTS = 1s
	rb.Write(&buffer.Frame{
		Timestamp:  1 * time.Second,
		IsKeyFrame: true,
		NALUs:      [][]byte{{0x05, 0x01, 0x02, 0x03}},
	})

	// 2. Forward P-frame at PTS = 1.08s
	time.Sleep(10 * time.Millisecond)
	rb.Write(&buffer.Frame{
		Timestamp:  1080 * time.Millisecond,
		IsKeyFrame: false,
		NALUs:      [][]byte{{0x01, 0x04}},
	})

	// 3. Reordered B-frame (backward regression in PTS = 1.04s)
	time.Sleep(10 * time.Millisecond)
	rb.Write(&buffer.Frame{
		Timestamp:  1040 * time.Millisecond, // PTS < lastPts
		IsKeyFrame: false,
		NALUs:      [][]byte{{0x01, 0x05}},
	})

	// 4. Another Keyframe at PTS = 2s
	time.Sleep(10 * time.Millisecond)
	rb.Write(&buffer.Frame{
		Timestamp:  2 * time.Second,
		IsKeyFrame: true,
		NALUs:      [][]byte{{0x05, 0x06}},
	})

	time.Sleep(100 * time.Millisecond)
	r.Stop()
	rb.Close()

	// Verify the generated file is structurally valid and can be validated
	camDir := filepath.Join(tempDir, "cam_bframe")
	entries, err := os.ReadDir(camDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected recorded files in %s, err: %v", camDir, err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".mp4") {
			filePath := filepath.Join(camDir, entry.Name())
			valid, valErr := ValidateFMP4File(filePath)
			if !valid || valErr != nil {
				t.Errorf("expected valid fMP4 with B-frames/regressions, got valid=%v, err=%v", valid, valErr)
			}
		}
	}
}

type mockRecorderBus struct {
	events map[string]int
	mu     sync.Mutex
}

func (m *mockRecorderBus) Publish(topic string, _ string, _ any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[topic]++
}

func (m *mockRecorderBus) Stop() {}

func TestRecorder_EventExclusivity_OnFailure(t *testing.T) {
	tempDir := t.TempDir()
	rb := buffer.NewRingBuffer(10)

	mockBus := &mockRecorderBus{events: make(map[string]int)}
	oldBus := registry.CurrentEventBus
	registry.CurrentEventBus = mockBus
	defer func() { registry.CurrentEventBus = oldBus }()

	// Set invalid SPS so fMP4 Init write fails immediately
	invalidSPS := []byte{0x00, 0x00}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, invalidSPS, pps)

	r := NewRecorder("cam_err_test", rb, tempDir, nil)
	time.Sleep(50 * time.Millisecond)

	rb.Write(&buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  0,
		NALUs:      [][]byte{{0x05, 0x01}},
	})

	time.Sleep(100 * time.Millisecond)
	r.Stop()
	rb.Close()

	mockBus.mu.Lock()
	failedCount := mockBus.events["recording_failed"]
	readyCount := mockBus.events["archive_segment_ready"]
	mockBus.mu.Unlock()

	if failedCount != 1 {
		t.Errorf("expected exactly 1 recording_failed event, got %d", failedCount)
	}
	if readyCount != 0 {
		t.Errorf("expected 0 archive_segment_ready events on error, got %d", readyCount)
	}
}

func BenchmarkRecorder_FMP4_Write_GOP(b *testing.B) {
	file := &discardWriteSeekCloser{Writer: io.Discard}

	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}

	init := &fmp4.Init{
		Tracks: []*fmp4.InitTrack{{
			ID:        1,
			TimeScale: 90000,
			Codec:     &fmp4.CodecH264{SPS: sps, PPS: pps},
		}},
	}
	_ = init.Marshal(file)

	// Prepare a GOP of 25 frames (1 second of video)
	var partSamples []*fmp4.PartSample
	for i := 0; i < 25; i++ {
		isKey := (i == 0)
		sample, _ := fmp4.NewPartSampleH26x(0, isKey, [][]byte{{0x01, 0x02, 0x03, 0x04}})
		sample.Duration = 90000 / 25
		partSamples = append(partSamples, sample)
	}

	part := &fmp4.Part{
		SequenceNumber: 1,
		Tracks: []*fmp4.PartTrack{{
			ID:       1,
			BaseTime: 0,
			Samples:  partSamples,
		}},
	}

	b.ResetTimer()
	b.ReportAllocs()
	var seq uint32
	for b.Loop() {
		seq++
		part.SequenceNumber = seq
		part.Tracks[0].BaseTime = uint64(seq) * 90000
		_ = part.Marshal(file)
	}
}
