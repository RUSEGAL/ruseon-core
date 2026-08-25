// Package grpc implements high-performance gRPC streaming services for raw H.264/H.265 video frame
// extraction (for external AI inference microservices) and bidirectional metadata ingestion.
package grpc

import (
	"fmt"
	"io"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/RUSEGAL/ruseon-core/internal/mqtt"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

// Server implements pb.FrameServiceServer to expose low-latency gRPC frame streams and ingest AI detections.
type Server struct {
	pb.UnimplementedFrameServiceServer
	manager *stream.Manager
	grpcSrv *grpc.Server
	mqttPub *mqtt.Publisher
}

// NewServer creates a new gRPC Server instance, optionally configuring TLS transport credentials if certFile/keyFile are provided.
func NewServer(manager *stream.Manager, mqttPub *mqtt.Publisher, certFile, keyFile string) *Server {
	var opts []grpc.ServerOption
	if certFile != "" && keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load gRPC TLS credentials")
		}
		opts = append(opts, grpc.Creds(creds))
	}

	return &Server{
		manager: manager,
		grpcSrv: grpc.NewServer(opts...),
		mqttPub: mqttPub,
	}
}

// Start binds a TCP listener to addr and serves incoming gRPC RPC connections.
func (s *Server) Start(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	pb.RegisterFrameServiceServer(s.grpcSrv, s)
	log.Info().Str("addr", addr).Msg("Starting gRPC Frame Extractor API")
	return s.grpcSrv.Serve(lis)
}

// Stop gracefully stops the gRPC server and closes active client streams.
func (s *Server) Stop() {
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
}

// StreamFrames handles client subscriptions to raw video frames for a given camera ID.
// Codec parameter sets (VPS/SPS/PPS) are prepended before keyframes to guarantee decodability.
func (s *Server) StreamFrames(req *pb.StreamRequest, srv pb.FrameService_StreamFramesServer) error {
	st, exists := s.manager.GetStream(req.CameraId)
	if !exists {
		return fmt.Errorf("camera %s not found", req.CameraId)
	}

	// Подписываемся на RingBuffer камеры
	reader := st.GetRingBuffer().NewReader()
	defer reader.Close()
	
	log.Info().Str("camera_id", req.CameraId).Msg("gRPC client subscribed to stream frames")

	// Reusable payload buffer per gRPC client stream (Zero-Alloc hot path, saving ~500 slice allocations/sec)
	payload := make([]byte, 0, 64*1024)

	for {
		frame, err := reader.ReadContext(srv.Context())
		if err != nil || frame == nil {
			if srv.Context().Err() != nil {
				log.Info().Str("camera_id", req.CameraId).Msg("gRPC client disconnected")
				return nil
			}
			return fmt.Errorf("stream closed")
		}

		params := st.GetRingBuffer().GetCodecParams()
		codec := "H264"
		if params != nil && len(params.VPS) > 0 {
			codec = "H265"
		}

		payload = payload[:0]

		// Отправляем параметры кодека перед каждым ключевым кадром для гарантии декодирования
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

		// Добавляем сами NAL-юниты кадра
		for _, nalu := range frame.NALUs {
			payload = append(payload, 0, 0, 0, 1)
			payload = append(payload, nalu...)
		}

		resp := &pb.FrameResponse{
			Codec:      codec,
			IsKeyframe: frame.IsKeyFrame,
			Pts:        frame.Timestamp.Milliseconds(),
			Payload:    payload,
		}

		if err := srv.Send(resp); err != nil {
			log.Error().Err(err).Str("camera_id", req.CameraId).Msg("Failed to send frame to gRPC client")
			return err
		}

		// Обновляем статистику трафика потока
		st.AddBytesSent(uint64(len(payload)))
	}
}

// PushMetadata receives client-streamed AI detections and broadcasts them to active viewers and MQTT subscribers.
func (s *Server) PushMetadata(srv pb.FrameService_PushMetadataServer) error {
	for {
		req, err := srv.Recv()
		if err != nil {
			if err == io.EOF {
				return srv.SendAndClose(&pb.MetadataResponse{
					Success: true,
					Message: "Metadata stream closed",
				})
			}
			return err
		}

		st, exists := s.manager.GetStream(req.CameraId)
		if !exists {
			continue
		}

		st.GetMetadataBroadcaster().Broadcast(req)

		// Отправляем в глобальную MQTT очередь (lock-free), если MQTT включен
		if s.mqttPub != nil {
			s.mqttPub.Push(req)
		}
	}
}
