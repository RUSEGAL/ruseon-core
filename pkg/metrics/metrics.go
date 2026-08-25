// Package metrics defines and exports Prometheus operational and telemetry metrics
// for streaming, ingest, archiving, viewer connections, and internal pipeline health.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveStreams tracks the number of currently active RTSP camera ingest sessions (labeled by camera_id).
	ActiveStreams = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ruseon_active_streams",
		Help: "The total number of currently active RTSP streams",
	}, []string{"camera_id"})

	// StreamReconnectsTotal counts the total number of RTSP stream disconnects and reconnect attempts (labeled by camera_id).
	StreamReconnectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_stream_reconnects_total",
		Help: "The total number of RTSP stream reconnects",
	}, []string{"camera_id"})

	// FramesReceivedTotal counts the total number of video frames (Access Units) received from RTSP cameras (labeled by camera_id).
	FramesReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_frames_received_total",
		Help: "The total number of frames received from all streams",
	}, []string{"camera_id"})

	// KeyFramesTotal counts the total number of IDR/key frames received across streams (labeled by camera_id).
	KeyFramesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_key_frames_total",
		Help: "The total number of I-Frames (key frames) received",
	}, []string{"camera_id"})

	// NetworkReceiveBytesTotal counts the cumulative raw video payload bytes ingested via RTSP (labeled by camera_id).
	NetworkReceiveBytesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_network_receive_bytes_total",
		Help: "The total number of bytes received from RTSP cameras",
	}, []string{"camera_id"})

	// DiskWriteBytesTotal counts the cumulative bytes written to MP4 archive files on storage (labeled by camera_id).
	DiskWriteBytesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_disk_write_bytes_total",
		Help: "The total number of bytes written to MP4 archives on disk",
	}, []string{"camera_id"})

	// ArchiveSegmentsWrittenTotal counts the number of completed MP4 archive chunks committed to storage (labeled by camera_id).
	ArchiveSegmentsWrittenTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_archive_segments_written_total",
		Help: "The total number of completed MP4 chunks successfully written",
	}, []string{"camera_id"})

	// ArchiveErrorsTotal counts I/O write errors encountered during MP4 archiving (labeled by camera_id).
	ArchiveErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_archive_errors_total",
		Help: "The total number of I/O errors during archiving",
	}, []string{"camera_id"})

	// RingbufferDropsTotal counts frames dropped from internal ring buffers when consumers lag behind (labeled by camera_id).
	RingbufferDropsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_ringbuffer_drops_total",
		Help: "The total number of frames dropped from the internal buffer due to slow consumers",
	}, []string{"camera_id"})

	// EventbusDropsTotal counts webhook notification events dropped due to saturated worker queues.
	EventbusDropsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ruseon_eventbus_drops_total",
		Help: "The total number of webhook events dropped due to full queues",
	})

	// WebRTCPeersActive tracks the number of currently active WebRTC (WHEP) peer connections (labeled by camera_id).
	WebRTCPeersActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ruseon_webrtc_peers_active",
		Help: "The current number of active WebRTC connections",
	}, []string{"camera_id"})

	// HLSRequestsTotal counts HTTP requests served for HLS playlists and media segments (labeled by camera_id).
	HLSRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ruseon_hls_requests_total",
		Help: "The total number of HTTP requests to HLS playlists and segments",
	}, []string{"camera_id"})
)

// DeleteCameraMetrics removes all time-series metric label pairs associated with the specified cameraID.
//
// This should be called whenever a camera is deleted to prevent metric cardinality and memory leaks
// in the Prometheus default registry.
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

