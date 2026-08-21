package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Подсистема захвата (RTSP)
	ActiveStreams = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ruseon_active_streams",
		Help: "The total number of currently active RTSP streams",
	}, []string{"camera_id"})
	StreamReconnectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_stream_reconnects_total",
		Help: "The total number of RTSP stream reconnects",
	}, []string{"camera_id"})
	FramesReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_frames_received_total",
		Help: "The total number of frames received from all streams",
	}, []string{"camera_id"})
	KeyFramesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_key_frames_total",
		Help: "The total number of I-Frames (key frames) received",
	}, []string{"camera_id"})
	NetworkReceiveBytesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_network_receive_bytes_total",
		Help: "The total number of bytes received from RTSP cameras",
	}, []string{"camera_id"})

	// Подсистема архивации (Storage / Recorder)
	DiskWriteBytesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_disk_write_bytes_total",
		Help: "The total number of bytes written to MP4 archives on disk",
	}, []string{"camera_id"})
	ArchiveSegmentsWrittenTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_archive_segments_written_total",
		Help: "The total number of completed MP4 chunks successfully written",
	}, []string{"camera_id"})
	ArchiveErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_archive_errors_total",
		Help: "The total number of I/O errors during archiving",
	}, []string{"camera_id"})

	// Ядро и стабильность (Buffers & EventBus)
	RingbufferDropsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_ringbuffer_drops_total",
		Help: "The total number of frames dropped from the internal buffer due to slow consumers",
	}, []string{"camera_id"})
	EventbusDropsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_eventbus_drops_total",
		Help: "The total number of webhook events dropped due to full queues",
	}) // Eventbus is global

	// Зрители (Viewers / HLS / WebRTC)
	WebRTCPeersActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ruseon_webrtc_peers_active",
		Help: "The current number of active WebRTC connections",
	}, []string{"camera_id"})
	HLSRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_hls_requests_total",
		Help: "The total number of HTTP requests to HLS playlists and segments",
	}, []string{"camera_id"})
)

// DeleteCameraMetrics удаляет все time series метрики, ассоциированные с удаленной камерой,
// предотвращая медленную утечку памяти в Prometheus Registry при частом удалении камер.
func DeleteCameraMetrics(cameraID string) {
	ActiveStreams.DeleteLabelValues(cameraID)
	StreamReconnectsTotal.DeleteLabelValues(cameraID)
	FramesReceivedTotal.DeleteLabelValues(cameraID)
	KeyFramesTotal.DeleteLabelValues(cameraID)
	NetworkReceiveBytesTotal.DeleteLabelValues(cameraID)
	DiskWriteBytesTotal.DeleteLabelValues(cameraID)
	ArchiveSegmentsWrittenTotal.DeleteLabelValues(cameraID)
	ArchiveErrorsTotal.DeleteLabelValues(cameraID)
	RingbufferDropsTotal.DeleteLabelValues(cameraID)
	WebRTCPeersActive.DeleteLabelValues(cameraID)
	HLSRequestsTotal.DeleteLabelValues(cameraID)
}

