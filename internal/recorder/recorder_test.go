package recorder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
)

func TestRecorder_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	rb := buffer.NewRingBuffer(10)
	
	// Set dummy SPS/PPS to satisfy recorder wait
	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps, pps)

	r := NewRecorder("cam1", rb, tempDir)
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

func BenchmarkRecorder_FMP4_Write_GOP(b *testing.B) {
	tempDir := b.TempDir()
	filePath := filepath.Join(tempDir, "bench.mp4")
	file, err := os.Create(filePath)
	if err != nil {
		b.Fatal(err)
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
	for i := 0; i < b.N; i++ {
		part.SequenceNumber = uint32(i + 1)
		part.Tracks[0].BaseTime = uint64(i * 90000)
		_ = part.Marshal(file)
	}
}
