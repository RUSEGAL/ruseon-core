// Package models defines core domain data models, state enumerations, and telemetry
// structures shared across RUSEON Core subsystems.
package models

// CameraState represents the operational connectivity status of a camera stream.
type CameraState string

const (
	// StateConnecting indicates the RTSP client is initiating a TCP/UDP socket handshake.
	StateConnecting CameraState = "connecting"
	// StateOnline indicates frames are arriving continuously within expected GOP intervals.
	StateOnline CameraState = "online"
	// StateDegraded indicates packet loss, decoding delays, or watchdog discontinuity warnings.
	StateDegraded CameraState = "degraded"
	// StateOffline indicates the RTSP stream disconnected and is awaiting backoff reconnection.
	StateOffline CameraState = "offline"
	// StateDisabled indicates ingest has been explicitly deactivated by an operator.
	StateDisabled CameraState = "disabled"
)

// CameraStats contains real-time runtime metrics and operational telemetry for an active camera stream.
type CameraStats struct {
	// State is the current connectivity lifecycle status.
	State CameraState `json:"state"`
	// BytesReceived is the cumulative raw payload bytes ingested from the RTSP source.
	BytesReceived uint64 `json:"bytes_received"`
	// BytesSent is the cumulative media bytes distributed to viewers (HLS, WebRTC, WebSocket, gRPC).
	BytesSent uint64 `json:"bytes_sent"`
	// Uptime is the continuous uptime in seconds since the latest successful RTSP connection.
	Uptime int64 `json:"uptime"`
	// Frames is the total count of video Access Units (frames) received.
	Frames uint64 `json:"frames"`
	// KeyFrames is the total count of IDR/key frames received.
	KeyFrames uint64 `json:"key_frames"`
	// Reconnects is the cumulative count of disconnect/reconnect cycles.
	Reconnects uint64 `json:"reconnects"`
	// Codec is the detected video codec (e.g. "H264", "H265").
	Codec string `json:"codec"`
	// LastFrameTime is the Unix timestamp (seconds) of the most recently received frame.
	LastFrameTime int64 `json:"last_frame_time"`
	// LastKeyTime is the Unix timestamp (seconds) of the most recently received key frame.
	LastKeyTime int64 `json:"last_key_time"`
	// LastError stores the latest network, RTSP, or decoding error message.
	LastError string `json:"last_error"`
	// Bitrate is the calculated rolling bitrate in kilobits per second (kbps).
	Bitrate float64 `json:"bitrate"`
}
