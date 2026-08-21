package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/metrics"
)

// WHEPHandler обрабатывает подключение по WebRTC.
type WHEPHandler struct {
	streamID string
	rb       *buffer.RingBuffer
	mb       *stream.MetadataBroadcaster
	cfg      *config.Config
	engine   *Engine
}

func NewWHEPHandler(streamID string, rb *buffer.RingBuffer, mb *stream.MetadataBroadcaster, cfg *config.Config, engine ...*Engine) *WHEPHandler {
	var eng *Engine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}
	return &WHEPHandler{
		streamID: streamID,
		rb:       rb,
		mb:       mb,
		cfg:      cfg,
		engine:   eng,
	}
}

// HandleOffer принимает SDP Offer клиента, создает PeerConnection и возвращает SDP Answer.
func (h *WHEPHandler) HandleOffer(_ context.Context, offerSDP string) (string, error) {
	params := h.rb.GetCodecParams()
	if params == nil || params.SPS == nil || params.PPS == nil {
		return "", errors.New("stream codec parameters not ready yet, please wait")
	}

	var iceServers []webrtc.ICEServer
	if h.cfg != nil && len(h.cfg.Server.WebRTC.ICEServers) > 0 {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       h.cfg.Server.WebRTC.ICEServers,
			Username:   h.cfg.Server.WebRTC.TURNUsername,
			Credential: h.cfg.Server.WebRTC.TURNPassword,
		})
	}

	iceTransportPolicy := webrtc.ICETransportPolicyAll
	if h.cfg != nil && h.cfg.Server.WebRTC.ICETransportPolicy == "relay" {
		iceTransportPolicy = webrtc.ICETransportPolicyRelay
	}

	config := webrtc.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: iceTransportPolicy,
	}

	var pc *webrtc.PeerConnection
	var err error
	if h.engine != nil {
		pc, err = h.engine.NewPeerConnection(config)
	} else {
		m := &webrtc.MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			return "", err
		}
		api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
		pc, err = api.NewPeerConnection(config)
	}
	if err != nil {
		return "", err
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	if err != nil {
		_ = pc.Close()
		return "", err
	}

	rtpSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		_ = pc.Close()
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
			_ = pc.Close()
			cancelPump()
		}
	})

	var isConnected atomic.Bool
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Info().Str("stream", h.streamID).Str("state", state.String()).Msg("WebRTC Connection State changed")
		if state == webrtc.PeerConnectionStateConnected {
			if !isConnected.Swap(true) {
				metrics.WebRTCPeersActive.WithLabelValues(h.streamID).Inc()
			}
		}
		if state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateFailed {
			if isConnected.Swap(false) {
				metrics.WebRTCPeersActive.WithLabelValues(h.streamID).Dec()
			}
		}
	})

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}
	
	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		if d.Label() == "metadata" {
			log.Info().Str("stream", h.streamID).Msg("Client opened metadata DataChannel")
			go h.pumpMetadata(pumpCtx, d)
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return "", err
	}

	// Ждем ICE gathering максимум 15 секунд (стандартный таймаут для STUN)
	select {
	case <-gatherComplete:
	case <-time.After(15 * time.Second):
		log.Warn().Str("stream", h.streamID).Msg("WebRTC ICE gathering timed out, proceeding with partial candidates")
	}

	go h.pumpFrames(pumpCtx, pc, videoTrack)

	return pc.LocalDescription().SDP, nil
}

func (h *WHEPHandler) pumpMetadata(ctx context.Context, dc *webrtc.DataChannel) {
	// Ждем, пока DataChannel откроется
	opened := make(chan struct{})
	dc.OnOpen(func() {
		close(opened)
	})

	select {
	case <-opened:
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second): // Таймаут на открытие
		log.Warn().Str("stream", h.streamID).Msg("DataChannel for metadata didn't open in time")
		return
	}

	sub := h.mb.Subscribe()
	defer h.mb.Unsubscribe(sub)

	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-sub.C:
			if !ok {
				return
			}
			
			// Формируем JSON
			data, err := json.Marshal(req)
			if err != nil {
				continue
			}
			
			if err := dc.SendText(string(data)); err != nil {
				log.Debug().Err(err).Str("stream", h.streamID).Msg("Failed to send metadata via DataChannel")
				return
			}
		}
	}
}

const (
	defaultFrameDuration = 40 * time.Millisecond // 25 FPS fallback
	minFrameDuration     = time.Millisecond          // 1000 FPS upper bound
	maxFrameDuration     = time.Second              // 1 FPS lower bound / gap clamp
)

// calculateFrameDuration computes dynamic frame duration from PTS delta with safety clamping.
func calculateFrameDuration(currentPTS time.Duration, lastPTS time.Duration, hasLastPTS bool) (time.Duration, time.Duration, bool) {
	if !hasLastPTS {
		return defaultFrameDuration, currentPTS, true
	}
	delta := currentPTS - lastPTS
	if delta <= 0 || delta > maxFrameDuration {
		// Clock reset, backward jump, reconnect, or excessive gap: fallback to safe default
		return defaultFrameDuration, currentPTS, true
	}
	if delta < minFrameDuration {
		delta = minFrameDuration
	}
	return delta, currentPTS, true
}

func (h *WHEPHandler) pumpFrames(ctx context.Context, pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) {
	reader := h.rb.NewReader()
	defer reader.Close()
	defer pc.Close()

	// Wait for the first keyframe to send SPS/PPS inline if needed
	params := h.rb.GetCodecParams()
	var sps, pps []byte
	if params != nil {
		sps, pps = params.SPS, params.PPS
	}

	var lastPTS time.Duration
	var hasLastPTS bool

	// Reusable buffer per WebRTC viewer goroutine (Zero-Alloc hot path)
	annexB := make([]byte, 0, 64*1024)

	for {
		frame, err := reader.ReadContext(ctx)
		if err != nil || frame == nil {
			return
		}

		var duration time.Duration
		duration, lastPTS, hasLastPTS = calculateFrameDuration(frame.Timestamp, lastPTS, hasLastPTS)

		annexB = annexB[:0]
		
		if frame.IsKeyFrame {
			if sps == nil || pps == nil {
				if p := h.rb.GetCodecParams(); p != nil {
					sps, pps = p.SPS, p.PPS
				}
			}
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

		if writeErr := track.WriteSample(media.Sample{
			Data:     annexB,
			Duration: duration,
		}); writeErr != nil {
			log.Error().Err(writeErr).Str("stream", h.streamID).Msg("WebRTC track write error")
			return
		}
	}
}
