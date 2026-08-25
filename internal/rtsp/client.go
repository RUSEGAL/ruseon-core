// Package rtsp provides an RTSP/RTP media client wrapper around gortsplib with support for
// H.264/H.265 video demuxing, automatic dial rate limiting, and 64-bit RTP timestamp unwrapping.
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

// OnFrameCallback is invoked on every fully assembled video frame (Access Unit).
type OnFrameCallback func(nalus [][]byte, pts time.Duration, isKeyFrame bool)

// OnParamsCallback is invoked when SPS/PPS (H.264) or VPS/SPS/PPS (H.265) parameter sets are extracted.
type OnParamsCallback func(vps, sps, pps []byte)

// dialSemaphore limits concurrent camera RTSP connection handshakes (protects against thundering herds).
var dialSemaphore = make(chan struct{}, 10)

// Client manages an RTSP ingest connection to a remote IP camera using gortsplib.
//
// Concurrency & Lifecycle:
//   - Thread-safe Close() can be called concurrently to cancel an active Start() loop.
//   - Start() blocks until the remote camera disconnects, encounters a fatal network error, or the ctx is cancelled.
type Client struct {
	client *gortsplib.Client
	url    string

	mu        sync.Mutex
	startDone bool
	closed    bool
}

// NewClient creates and configures a new RTSP Client instance.
//
// transportStr specifies the underlying transport: "tcp", "udp", or "auto" (defaults to TCP for reliable packet delivery).
func NewClient(id, url string, transportStr string) *Client {
	c := &Client{
		url: url,
		client: &gortsplib.Client{
			// Allow any available port for receiving RTP/RTCP packets
			AnyPortEnable: true,
		},
	}

	switch transportStr {
	case "udp":
		t := gortsplib.TransportUDP
		c.client.Transport = &t
	default:
		// "tcp", "auto" or any other value defaults to TCP
		t := gortsplib.TransportTCP
		c.client.Transport = &t
	}

	// Intercept packet loss and decode errors for telemetry logging
	c.client.OnPacketLost = func(err error) {
		log.Warn().Str("id", id).Str("url", url).Msg(err.Error())
	}
	c.client.OnDecodeError = func(err error) {
		log.Warn().Str("id", id).Str("url", url).Msg(err.Error())
	}

	return c
}

// Close disconnects the RTSP client and releases network sockets idempotently.
func (c *Client) Close() {
	c.mu.Lock()
	c.closed = true
	canClose := c.startDone
	c.mu.Unlock()

	if canClose {
		c.client.Close()
	}
}

// Start initiates the RTSP handshake (DESCRIBE, SETUP, PLAY) and begins RTP packet reception.
//
// Flow and concurrency guarantees:
//   - Acquires dialSemaphore during TCP/RTSP handshake to prevent camera load spikes.
//   - Detects H.264 or H.265 video tracks and hooks up the respective RTP packet decoders.
//   - Unwraps 32-bit 90kHz RTP timestamps into continuous 64-bit presentation durations.
//   - Blocks until ctx is cancelled, the camera terminates the RTSP session, or a network failure occurs.
func (c *Client) Start(ctx context.Context, onFrame OnFrameCallback, onParams OnParamsCallback) error {
	u, err := base.ParseURL(c.url)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	// Acquire dial semaphore before connecting
	dialSemaphore <- struct{}{}
	semaphoreReleased := false
	releaseSemaphore := func() {
		if !semaphoreReleased {
			<-dialSemaphore
			semaphoreReleased = true
		}
	}
	defer releaseSemaphore()

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

	// Setup media tracks
	err = c.client.SetupAll(session.BaseURL, session.Medias)
	if err != nil {
		return fmt.Errorf("failed to setup all: %w", err)
	}
	
	// Release dial semaphore now that handshake is complete
	releaseSemaphore()

	tsUnwrapper := NewTimestampUnwrapper()

	// Locate video track (H.264 or H.265) and subscribe to RTP packets
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
						unwrappedTS := tsUnwrapper.Unwrap(pkt.Timestamp)
						pts := RTP90kToDuration(unwrappedTS)

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
						unwrappedTS := tsUnwrapper.Unwrap(pkt.Timestamp)
						pts := RTP90kToDuration(unwrappedTS)

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

	// Await session completion or context cancellation
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Recovered from panic in RTSP Client Wait")
				errChan <- fmt.Errorf("panic in wait: %v", r)
			}
		}()
		errChan <- c.client.Wait()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		c.client.Close()
		<-errChan
		return ctx.Err()
	}
}
