package grpc

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/RUSEGAL/ruseon-core/v2/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/grpc/pb"
)

func BenchmarkGRPC_FrameResponseMarshal(b *testing.B) {
	payload := make([]byte, 8192) // typical 8KB P-frame
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	resp := &pb.FrameResponse{
		Codec:      "H264",
		IsKeyframe: false,
		Pts:        123456789,
		Payload:    payload,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = proto.Marshal(resp)
	}
}

func BenchmarkGRPC_PayloadAssembly(b *testing.B) {
	rb := buffer.NewRingBuffer(10)
	defer rb.Close()

	sps := []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	rb.SetParams(nil, sps, pps)

	nalu1 := make([]byte, 2048)
	nalu2 := make([]byte, 2048)
	frame := &buffer.Frame{
		IsKeyFrame: true,
		Timestamp:  time.Duration(100) * time.Millisecond,
		NALUs:      [][]byte{nalu1, nalu2},
	}

	payload := make([]byte, 0, 64*1024)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		params := rb.GetCodecParams()
		codec := "H264"
		if params != nil && len(params.VPS) > 0 {
			codec = "H265"
		}
		_ = codec

		payload = payload[:0]

		if frame.IsKeyFrame && params != nil {
			if len(params.VPS) > 0 {
				payload = append(payload, 0, 0, 0, 1)
				payload = append(payload, params.VPS...)
			}
			if len(params.SPS) > 0 {
				payload = append(payload, 0, 0, 0, 1)
				payload = append(payload, params.SPS...)
			}
			if len(params.PPS) > 0 {
				payload = append(payload, 0, 0, 0, 1)
				payload = append(payload, params.PPS...)
			}
		}

		for _, nalu := range frame.NALUs {
			payload = append(payload, 0, 0, 0, 1)
			payload = append(payload, nalu...)
		}
	}
}
