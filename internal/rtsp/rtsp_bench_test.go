package rtsp

import (
	"testing"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// BenchmarkRTPDecode замеряет оверхед на парсинг RTP -> NAL
func BenchmarkRTPDecode(b *testing.B) {
	f := &format.H264{
		PayloadTyp:        96,
		SPS:               []byte{0x07, 0x01, 0x02, 0x03},
		PPS:               []byte{0x08, 0x01},
		PacketizationMode: 1,
	}
	dec, err := f.CreateDecoder()
	if err != nil {
		b.Fatal(err)
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1,
			Timestamp:      90000,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04}, // Dummy payload
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = dec.Decode(pkt)
	}
}
