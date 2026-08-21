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

	"github.com/RUSEGAL/ruseon-core/pkg/config"
)

// Engine инкапсулирует переиспользуемый WebRTC API, предсгенерированные сертификаты и UDP Muxer.
type Engine struct {
	api         *webrtc.API
	certificate *webrtc.Certificate
	udpListener *net.UDPConn
	cfg         *config.Config
	mu          sync.Mutex
}

// NewEngine создает и инициализирует высокопроизводительный WebRTC Engine.
func NewEngine(cfg *config.Config) (*Engine, error) {
	// 1. Предгенерация DTLS сертификата (0 ms при последующих подключениях)
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	cert, err := webrtc.GenerateCertificate(sk)
	if err != nil {
		return nil, err
	}

	// 2. MediaEngine с регистрацией стандартных кодеков
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	// 3. SettingEngine: UDP Muxing и NAT 1:1 IPs
	settingEngine := webrtc.SettingEngine{}

	var udpListener *net.UDPConn
	if cfg != nil && cfg.Server.WebRTC.ListenPort > 0 {
		l, err := net.ListenUDP("udp", &net.UDPAddr{Port: cfg.Server.WebRTC.ListenPort})
		if err != nil {
			log.Warn().Err(err).Int("port", cfg.Server.WebRTC.ListenPort).Msg("Failed to bind WebRTC UDP Mux port, falling back to dynamic ports")
		} else {
			udpListener = l
			// Оптимизация системных буферов сокетов UDP для сглаживания всплесков трафика
			_ = udpListener.SetReadBuffer(4 * 1024 * 1024)
			_ = udpListener.SetWriteBuffer(4 * 1024 * 1024)
			udpMux := webrtc.NewICEUDPMux(nil, udpListener)
			settingEngine.SetICEUDPMux(udpMux)
			log.Info().Int("port", cfg.Server.WebRTC.ListenPort).Msg("WebRTC UDP Muxer listening")
		}
	}

	if cfg != nil && len(cfg.Server.WebRTC.NAT1To1IPs) > 0 {
		settingEngine.SetNAT1To1IPs(cfg.Server.WebRTC.NAT1To1IPs, webrtc.ICECandidateTypeHost)
		log.Info().Strs("ips", cfg.Server.WebRTC.NAT1To1IPs).Msg("WebRTC configured with NAT 1:1 IPs")
	}

	// 4. Облегченный Interceptor Registry: компактный NACK responder (256 пакетов = ~500-1000мс истории)
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

// NewPeerConnection создает PeerConnection с предсгенерированным сертификатом и переиспользуемым API.
func (e *Engine) NewPeerConnection(baseConfig webrtc.Configuration) (*webrtc.PeerConnection, error) {
	if len(baseConfig.Certificates) == 0 && e.certificate != nil {
		baseConfig.Certificates = []webrtc.Certificate{*e.certificate}
	}
	return e.api.NewPeerConnection(baseConfig)
}

// Close закрывает UDP listener при остановке сервера.
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
