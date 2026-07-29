package rtsp

import (
	"context"
	"fmt"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// OnFrameCallback вызывается при получении полного кадра (Access Unit).
type OnFrameCallback func(nalus [][]byte, pts time.Duration, isKeyFrame bool)

// OnParamsCallback вызывается при получении параметров кодека.
type OnParamsCallback func(sps, pps []byte)

// Client обертка над gortsplib.Client
type Client struct {
	client *gortsplib.Client
	url    string
}

// NewClient создает новый RTSP клиент.
func NewClient(url string) *Client {
	return &Client{
		url: url,
		client: &gortsplib.Client{
			// Разрешаем любой порт для приема RTP/RTCP
			AnyPortEnable: true,
		},
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

	// Ищем видео трек (H264) и подписываемся на него
	for _, media := range session.Medias {
		for _, forma := range media.Formats {
			if f, ok := forma.(*format.H264); ok {
				// Создаем декодер RTP -> H264 NALUs
				rtpDec, err := f.CreateDecoder()
				if err != nil {
					return fmt.Errorf("failed to create H264 decoder: %w", err)
				}
				
				// Передаем параметры кодека, если они есть
				sps, pps := f.SafeParams()
				if sps != nil && pps != nil {
					onParams(sps, pps)
				}

				c.client.OnPacketRTP(media, f, func(pkt *rtp.Packet) {
					// Декодируем RTP пакет в сырые NALU
					nalus, err := rtpDec.Decode(pkt)
					if err == nil && len(nalus) > 0 {
						isKeyFrame := false
						
						// Вычисляем PTS (RTP timestamp для H264 использует 90000 Hz)
						pts := time.Duration(pkt.Timestamp) * time.Second / 90000

						// Проверяем тип NALU. IDR (тип 5) означает ключевой кадр.
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
