package webrtc

import (
	"context"
	"errors"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
)

// WHEPHandler обрабатывает подключение по WebRTC.
type WHEPHandler struct {
	streamID string
	rb       *buffer.RingBuffer
}

func NewWHEPHandler(streamID string, rb *buffer.RingBuffer) *WHEPHandler {
	return &WHEPHandler{
		streamID: streamID,
		rb:       rb,
	}
}

// HandleOffer принимает SDP Offer клиента, создает PeerConnection и возвращает SDP Answer.
func (h *WHEPHandler) HandleOffer(_ context.Context, offerSDP string) (string, error) {
	_, sps, pps := h.rb.GetParams()
	if sps == nil || pps == nil {
		return "", errors.New("stream codec parameters not ready yet, please wait")
	}

	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return "", err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return "", err
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	if err != nil {
		pc.Close()
		return "", err
	}

	rtpSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		pc.Close()
		return "", err
	}

	// Read incoming RTCP packets
	// Before these packets are read they are intercepted and routed by Pion
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Отвязываем контекст от HTTP-запроса, так как он завершится после отправки ответа.
	// Вместо этого создаем локальный контекст, который отменится при закрытии соединения.
	pumpCtx, cancelPump := context.WithCancel(context.Background())

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Info().Str("stream", h.streamID).Str("state", connectionState.String()).Msg("WebRTC ICE State")
		if connectionState == webrtc.ICEConnectionStateClosed ||
			connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateDisconnected {
			pc.Close()
			cancelPump()
		}
	})

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", err
	}

	<-gatherComplete

	go h.pumpFrames(pumpCtx, pc, videoTrack)

	return pc.LocalDescription().SDP, nil
}

func (h *WHEPHandler) pumpFrames(ctx context.Context, pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) {
	reader := h.rb.NewReader()
	defer reader.Close()
	defer pc.Close()

	// Wait for the first keyframe to send SPS/PPS inline if needed
	_, sps, pps := h.rb.GetParams()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		frame := reader.Read()
		if frame == nil {
			return
		}

		annexB := make([]byte, 0, 1024*100) // pre-allocate
		
		if frame.IsKeyFrame {
			// На ключевом кадре внедряем SPS/PPS (Annex-B)
			if sps != nil {
				annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
				annexB = append(annexB, sps...)
			}
			if pps != nil {
				annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
				annexB = append(annexB, pps...)
			}
		}

		for _, nalu := range frame.NALUs {
			annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
			annexB = append(annexB, nalu...)
		}

		err := track.WriteSample(media.Sample{
			Data:     annexB,
			Duration: time.Second / 25, // Assume 25fps for playback pacing
		})

		if err != nil {
			log.Error().Err(err).Str("stream", h.streamID).Msg("WebRTC track write error")
			return
		}
	}
}
