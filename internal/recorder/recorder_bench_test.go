package recorder

import (
	"io"
	"testing"

	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
)

type discardWriteSeekCloser struct {
	io.Writer
}

func (d *discardWriteSeekCloser) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (d *discardWriteSeekCloser) Close() error                                 { return nil }
func (d *discardWriteSeekCloser) DropCache() error                             { return nil }

// BenchmarkWriteGOP собирает 25 сырых кадров в fMP4 и пишет их (эмуляция 1 секунды видео).
func BenchmarkWriteGOP(b *testing.B) {
	nalu := make([]byte, 1024)

	var samples []*fmp4.PartSample
	for i := 0; i < 25; i++ {
		sample, _ := fmp4.NewPartSampleH26x(3600, i == 0, [][]byte{nalu}) // 3600 = 90000 / 25
		samples = append(samples, sample)
	}

	file := &discardWriteSeekCloser{Writer: io.Discard}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		part := &fmp4.Part{
			SequenceNumber: uint32(i),
			Tracks: []*fmp4.PartTrack{{
				ID:       1,
				BaseTime: uint64(i * 90000),
				Samples:  samples,
			}},
		}
		_ = part.Marshal(file)
	}
}
