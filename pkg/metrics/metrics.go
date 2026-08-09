package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Подсистема захвата (RTSP)
	ActiveStreams = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ruseon_active_streams",
		Help: "The total number of currently active RTSP streams",
	})
	StreamReconnectsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_stream_reconnects_total",
		Help: "The total number of RTSP stream reconnects",
	})
	FramesReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_frames_received_total",
		Help: "The total number of frames received from all streams",
	})
	KeyFramesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_key_frames_total",
		Help: "The total number of I-Frames (key frames) received",
	})
	NetworkReceiveBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_network_receive_bytes_total",
		Help: "The total number of bytes received from RTSP cameras",
	})

	// Подсистема архивации (Storage / Recorder)
	DiskWriteBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_disk_write_bytes_total",
		Help: "The total number of bytes written to MP4 archives on disk",
	})
	ArchiveSegmentsWrittenTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_archive_segments_written_total",
		Help: "The total number of completed MP4 chunks successfully written",
	})
	ArchiveErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_archive_errors_total",
		Help: "The total number of I/O errors during archiving",
	})

	// Ядро и стабильность (Buffers & EventBus)
	RingbufferDropsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_ringbuffer_drops_total",
		Help: "The total number of frames dropped from the internal buffer due to slow consumers",
	})
	EventbusDropsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_eventbus_drops_total",
		Help: "The total number of webhook events dropped due to full queues",
	})

	// Зрители (Viewers / HLS / WebRTC)
	WebRTCPeersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ruseon_webrtc_peers_active",
		Help: "The current number of active WebRTC connections",
	})
	HLSRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_hls_requests_total",
		Help: "The total number of HTTP requests to HLS playlists and segments",
	})
)
