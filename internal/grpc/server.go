package grpc

import (
	"fmt"
	"io"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/RUSEGAL/ruseon-core/internal/mqtt"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

// Server реализует gRPC API для трансляции сырых видеокадров.
type Server struct {
	pb.UnimplementedFrameServiceServer
	manager *stream.Manager
	grpcSrv *grpc.Server
	mqttPub *mqtt.Publisher
}

// NewServer создает новый экземпляр gRPC сервера.
func NewServer(manager *stream.Manager, mqttPub *mqtt.Publisher) *Server {
	return &Server{
		manager: manager,
		grpcSrv: grpc.NewServer(),
		mqttPub: mqttPub,
	}
}

// Start запускает gRPC сервер на указанном порту.
func (s *Server) Start(port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	pb.RegisterFrameServiceServer(s.grpcSrv, s)
	log.Info().Str("port", port).Msg("Starting gRPC Frame Extractor API")
	return s.grpcSrv.Serve(lis)
}

// Stop останавливает gRPC сервер.
func (s *Server) Stop() {
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
}

// StreamFrames реализует RPC метод для подписки на кадры конкретной камеры.
func (s *Server) StreamFrames(req *pb.StreamRequest, srv pb.FrameService_StreamFramesServer) error {
	st, exists := s.manager.GetStream(req.CameraId)
	if !exists {
		return fmt.Errorf("camera %s not found", req.CameraId)
	}

	// Подписываемся на RingBuffer камеры
	reader := st.GetRingBuffer().NewReader()
	defer reader.Close()
	
	log.Info().Str("camera_id", req.CameraId).Msg("gRPC client subscribed to stream frames")

	for {
		if srv.Context().Err() != nil {
			log.Info().Str("camera_id", req.CameraId).Msg("gRPC client disconnected")
			return nil
		}

		frame := reader.Read()
		if frame == nil {
			return fmt.Errorf("stream closed")
		}

			// Определяем кодек
			codec := "H264"
			vps, _, _ := st.GetRingBuffer().GetParams()
			if len(vps) > 0 {
				codec = "H265"
			}

			// Собираем полезную нагрузку в формате Annex B (с префиксом 0x00 0x00 0x00 0x01)
			// Это стандартный формат, который ожидают FFmpeg, OpenCV и большинство AI-моделей
			var payload []byte
			
			// Отправляем параметры кодека перед каждым ключевым кадром для гарантии декодирования
			if frame.IsKeyFrame {
				if len(vps) > 0 {
					payload = append(payload, []byte{0, 0, 0, 1}...)
					payload = append(payload, vps...)
				}
				_, sps, pps := st.GetRingBuffer().GetParams()
				if len(sps) > 0 {
					payload = append(payload, []byte{0, 0, 0, 1}...)
					payload = append(payload, sps...)
				}
				if len(pps) > 0 {
					payload = append(payload, []byte{0, 0, 0, 1}...)
					payload = append(payload, pps...)
				}
			}

			// Добавляем сами NAL-юниты кадра
			for _, nalu := range frame.NALUs {
				payload = append(payload, []byte{0, 0, 0, 1}...)
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

// PushMetadata принимает поток метаданных от ИИ-модулей и рассылает их зрителям.
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
