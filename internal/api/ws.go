package api

import (
	"context"
	"encoding/binary"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS handled by upper middleware
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 64 * 1024,
}

// StreamWS handles binary WebCodecs streaming over WebSocket.
// Protocol Specification:
//
// 1. Initial Config Header (Packet Type 0x01):
//    [0x01]                 : 1 byte (Header identifier)
//    [CodecType]            : 1 byte (0x01 = H264, 0x02 = H265)
//    [VPS Length]           : 2 bytes uint16 (BigEndian)
//    [VPS Data]             : N bytes
//    [SPS Length]           : 2 bytes uint16 (BigEndian)
//    [SPS Data]             : N bytes
//    [PPS Length]           : 2 bytes uint16 (BigEndian)
//    [PPS Data]             : N bytes
//
// 2. Video Data Packet (Packet Type 0x02):
//    [0x02]                 : 1 byte (Data identifier)
//    [IsKeyFrame]           : 1 byte (0x01 = Keyframe, 0x00 = Delta)
//    [Timestamp Microsecs]  : 8 bytes uint64 (BigEndian)
//    [Annex-B NALU Payload] : Remaining bytes (00 00 00 01 <NALU> ...)
func (h *Handler) StreamWS(c *gin.Context) {
	id := c.Param("id")
	h.tracker.Mark(c.ClientIP(), id)

	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Wait up to 3 seconds for codec parameters (SPS/PPS) if camera is newly connected
	var vps, sps, pps []byte
	for i := 0; i < 30; i++ {
		vps, sps, pps = st.GetRingBuffer().GetParams()
		if len(sps) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	codecType := byte(1) // H.264 default
	if len(vps) > 0 {
		codecType = 2 // H.265 / HEVC
	}

	// 1. Build and send Header Packet
	headerSize := 1 + 1 + 2 + len(vps) + 2 + len(sps) + 2 + len(pps)
	headerBuf := make([]byte, headerSize)
	headerBuf[0] = 0x01
	headerBuf[1] = codecType
	offset := 2

	binary.BigEndian.PutUint16(headerBuf[offset:], uint16(len(vps)))
	offset += 2
	copy(headerBuf[offset:], vps)
	offset += len(vps)

	binary.BigEndian.PutUint16(headerBuf[offset:], uint16(len(sps)))
	offset += 2
	copy(headerBuf[offset:], sps)
	offset += len(sps)

	binary.BigEndian.PutUint16(headerBuf[offset:], uint16(len(pps)))
	offset += 2
	copy(headerBuf[offset:], pps)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.BinaryMessage, headerBuf); err != nil {
		log.Debug().Err(err).Str("id", id).Msg("Client disconnected before WebCodecs header")
		return
	}

	// 2. Subscribe to RingBuffer
	sub := st.GetRingBuffer().Subscribe()
	defer sub.Close()

	// Configure heartbeat and reader loop to detect disconnects
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go func() {
		defer cancel()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	annexBHeader := []byte{0x00, 0x00, 0x00, 0x01}
	syncedKeyFrame := false

	// Annex-B reusable packet buffer
	for {
		select {
		case <-ctx.Done():
			return

		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case frame, ok := <-sub.C:
			if !ok {
				return
			}

			// Ensure we start streaming strictly from an I-Frame for clean decoder initialization
			if !syncedKeyFrame {
				if !frame.IsKeyFrame {
					continue
				}
				syncedKeyFrame = true
			}

			// Calculate total payload size
			totalPayloadLen := 0
			for _, nalu := range frame.NALUs {
				totalPayloadLen += len(annexBHeader) + len(nalu)
			}

			// Packet structure: [0x02][IsKeyFrame 1B][Timestamp 8B][Payload...]
			packet := make([]byte, 1+1+8+totalPayloadLen)
			packet[0] = 0x02
			if frame.IsKeyFrame {
				packet[1] = 0x01
			} else {
				packet[1] = 0x00
			}
			binary.BigEndian.PutUint64(packet[2:10], uint64(frame.Timestamp.Microseconds()))

			writePos := 10
			for _, nalu := range frame.NALUs {
				copy(packet[writePos:], annexBHeader)
				writePos += len(annexBHeader)
				copy(packet[writePos:], nalu)
				writePos += len(nalu)
			}

			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if err := conn.WriteMessage(websocket.BinaryMessage, packet); err != nil {
				return
			}

			st.AddBytesSent(uint64(len(packet)))
		}
	}
}
