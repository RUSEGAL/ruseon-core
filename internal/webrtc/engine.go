// Package webrtc provides high-throughput WebRTC WHEP (WebRTC-HTTP Egress Protocol) streaming,
// pre-warmed DTLS certificate handling, UDP port multiplexing, and zero-allocation sendmmsg batching.
package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net"
	"sync"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

// Engine encapsulates reusable Pion WebRTC instances, pre-generated DTLS certificates,
// and single-port UDP ICE multiplexing.
//
// Concurrency & Performance:
//   - Pre-generates DTLS ECDSA P-256 certificates on startup, eliminating ~15-30ms CPU crypto spikes per client.
//   - Shares a single UDP listener port across thousands of concurrent WebRTC sessions via ICEUDPMux.
//   - Adapts packet egress with kernel-level batching (sendmmsg on Linux).
type Engine struct {
	api         *webrtc.API
	certificate *webrtc.Certificate
	udpListener *net.UDPConn
	cfg         *config.Config
	mu          sync.Mutex
}

// NewEngine initializes and configures a global WebRTC Engine with optimal media codecs and network socket buffers.
func NewEngine(cfg *config.Config) (*Engine, error) {
	// 1. Pre-generate DTLS certificate for zero-latency handshake
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	cert, err := webrtc.GenerateCertificate(sk)
	if err != nil {
		return nil, err
	}

	// 2. MediaEngine with standard H.264/Opus codecs
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	// 3. SettingEngine: UDP Muxing and 1:1 NAT candidate mapping
	settingEngine := webrtc.SettingEngine{}

	var udpListener *net.UDPConn
	listenPort := 0
	if cfg != nil && cfg.Server.WebRTC.ListenPort > 0 {
		listenPort = cfg.Server.WebRTC.ListenPort
	}

	l, err := net.ListenUDP("udp", &net.UDPAddr{Port: listenPort})
	if err != nil {
		log.Warn().Err(err).Int("port", listenPort).Msg("Failed to bind WebRTC UDP Mux port, falling back to dynamic ports")
	} else {
		udpListener = l
		// Expand system socket buffer size to smooth packet burst spikes
		_ = udpListener.SetReadBuffer(4 * 1024 * 1024)
		_ = udpListener.SetWriteBuffer(4 * 1024 * 1024)
		batchConn := NewBatchingUDPMuxConn(udpListener)
		udpMux := webrtc.NewICEUDPMux(nil, batchConn)
		settingEngine.SetICEUDPMux(udpMux)
		actualPort := udpListener.LocalAddr().(*net.UDPAddr).Port
		log.Info().Int("port", actualPort).Msg("WebRTC UDP Muxer listening with adaptive sendmmsg batching")
	}

	if cfg != nil && len(cfg.Server.WebRTC.NAT1To1IPs) > 0 {
		settingEngine.SetNAT1To1IPs(cfg.Server.WebRTC.NAT1To1IPs, webrtc.ICECandidateTypeHost)
		log.Info().Strs("ips", cfg.Server.WebRTC.NAT1To1IPs).Msg("WebRTC configured with NAT 1:1 IPs")
	}

	// 4. Compact NACK responder interceptor (256 packets history buffer)
	ir := &interceptor.Registry{}
	if nackResponder, err := nack.NewResponderInterceptor(nack.ResponderSize(256)); err == nil {
		ir.Add(nackResponder)
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
		webrtc.WithInterceptorRegistry(ir),
	)

	return &Engine{
		api:         api,
		certificate: cert,
		udpListener: udpListener,
		cfg:         cfg,
	}, nil
}

// NewPeerConnection allocates and configures a new WebRTC PeerConnection injecting the pre-generated DTLS certificate.
func (e *Engine) NewPeerConnection(baseConfig webrtc.Configuration) (*webrtc.PeerConnection, error) {
	if len(baseConfig.Certificates) == 0 && e.certificate != nil {
		baseConfig.Certificates = []webrtc.Certificate{*e.certificate}
	}
	return e.api.NewPeerConnection(baseConfig)
}

// Close gracefully closes the UDP listener socket and frees multiplexing resources.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.udpListener != nil {
		err := e.udpListener.Close()
		e.udpListener = nil
		return err
	}
	return nil
}
