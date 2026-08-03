package rtsp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
	"github.com/rs/zerolog/log"
)

// OnFrameCallback вызывается при получении полного кадра (Access Unit).
type OnFrameCallback func(nalus [][]byte, pts time.Duration, isKeyFrame bool)

// OnParamsCallback вызывается при получении параметров кодека.
type OnParamsCallback func(vps, sps, pps []byte)

// Client обертка над gortsplib.Client
type Client struct {
	client *gortsplib.Client
	url    string

	mu        sync.Mutex
	startDone bool
	closed    bool
}

// NewClient создает новый RTSP клиент.
func NewClient(id, url string) *Client {
	transport := gortsplib.TransportTCP

	c := &Client{
		url: url,
		client: &gortsplib.Client{
			// Принудительно используем TCP для RTSP, чтобы избежать
			// потери UDP пакетов и появления "зеленых квадратов" (артефактов).
			Transport: &transport,
			// Разрешаем любой порт для приема RTP/RTCP
			AnyPortEnable: true,
		},
	}

	// Настраиваем перехват ошибок потерянных пакетов и декодирования
	c.client.OnPacketLost = func(err error) {
		log.Warn().Str("id", id).Str("url", url).Msg(err.Error())
	}
	c.client.OnDecodeError = func(err error) {
		log.Warn().Str("id", id).Str("url", url).Msg(err.Error())
	}

	return c
}

// Close закрывает соединение
func (c *Client) Close() {
	c.mu.Lock()
	c.closed = true
	canClose := c.startDone
	c.mu.Unlock()

	if canClose {
		c.client.Close()
	}
}

// Start подключается к камере и блокирует выполнение до отключения.
func (c *Client) Start(ctx context.Context, onFrame OnFrameCallback, onParams OnParamsCallback) error {
	u, err := base.ParseURL(c.url)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	err = c.client.Start(u.Scheme, u.Host)
	if err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	c.mu.Lock()
	c.startDone = true
	shouldClose := c.closed
	c.mu.Unlock()

	if shouldClose {
		c.client.Close()
		return fmt.Errorf("client was closed during start")
	}

	defer c.client.Close()

	session, _, err := c.client.Describe(u)
	if err != nil {
		return fmt.Errorf("failed to describe: %w", err)
	}

	// Настраиваем все медиа-треки (видео и аудио)
	err = c.client.SetupAll(session.BaseURL, session.Medias)
	if err != nil {
		return fmt.Errorf("failed to setup all: %w", err)
	}

	// Ищем видео трек (H264 или H265) и подписываемся на него
	for _, media := range session.Medias {
		for _, forma := range media.Formats {
			switch f := forma.(type) {
			case *format.H264:
				rtpDec, err := f.CreateDecoder()
				if err != nil {
					return fmt.Errorf("failed to create H264 decoder: %w", err)
				}

				sps, pps := f.SafeParams()
				if sps != nil && pps != nil {
					onParams(nil, sps, pps)
				}

				c.client.OnPacketRTP(media, f, func(pkt *rtp.Packet) {
					nalus, err := rtpDec.Decode(pkt)
					if err == nil && len(nalus) > 0 {
						isKeyFrame := false
						pts := time.Duration(pkt.Timestamp) * time.Second / 90000

						for _, nalu := range nalus {
							if len(nalu) > 0 {
								typ := nalu[0] & 0x1F
								if typ == 5 {
									isKeyFrame = true
									break
								}
							}
						}
						onFrame(nalus, pts, isKeyFrame)
					}
				})
			case *format.H265:
				rtpDec, err := f.CreateDecoder()
				if err != nil {
					return fmt.Errorf("failed to create H265 decoder: %w", err)
				}

				vps, sps, pps := f.SafeParams()
				if vps != nil && sps != nil && pps != nil {
					onParams(vps, sps, pps)
				}

				c.client.OnPacketRTP(media, f, func(pkt *rtp.Packet) {
					nalus, err := rtpDec.Decode(pkt)
					if err == nil && len(nalus) > 0 {
						isKeyFrame := false
						pts := time.Duration(pkt.Timestamp) * time.Second / 90000

						for _, nalu := range nalus {
							if len(nalu) > 0 {
								typ := (nalu[0] >> 1) & 0x3F
								// 16 to 21 are IRAP pictures (CRA, BLA, IDR)
								if typ >= 16 && typ <= 21 {
									isKeyFrame = true
									break
								}
							}
						}
						onFrame(nalus, pts, isKeyFrame)
					}
				})
			}
		}
	}

	_, err = c.client.Play(nil)
	if err != nil {
		return fmt.Errorf("failed to play: %w", err)
	}

	// Ожидаем завершения сессии или отмены контекста
	errChan := make(chan error, 1)
	go func() {
		errChan <- c.client.Wait()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
