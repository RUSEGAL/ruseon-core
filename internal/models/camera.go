package models

// CameraStats содержит статистику по камере.
type CameraStats struct {
	Connected     bool
	BytesReceived uint64
	BytesSent     uint64
	Uptime        int64
	Frames        uint64
	KeyFrames     uint64
	Reconnects    uint64
	Codec         string
}
