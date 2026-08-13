package models

type CameraState string

const (
	StateConnecting CameraState = "connecting"
	StateOnline     CameraState = "online"
	StateDegraded   CameraState = "degraded"
	StateOffline    CameraState = "offline"
	StateDisabled   CameraState = "disabled"
)

// CameraStats содержит статистику по камере.
type CameraStats struct {
	State         CameraState
	BytesReceived uint64
	BytesSent     uint64
	Uptime        int64
	Frames        uint64
	KeyFrames     uint64
	Reconnects    uint64
	Codec         string
	LastFrameTime int64
	LastKeyTime   int64
	LastError     string
	Bitrate       float64
}
