// Package buffer implements high-performance circular memory ring buffers and frame distribution
// pipelines for video frames and codec parameter sets.
package buffer

import (
	"time"
)

// Frame represents a single video Access Unit comprising one or more Network Abstraction Layer Units (NALUs).
type Frame struct {
	// Timestamp is the presentation duration offset of the frame relative to stream start.
	Timestamp time.Duration
	// IsKeyFrame is true if this frame contains an IDR/key picture (allowing decoders to start rendering).
	IsKeyFrame bool
	// NALUs contains the raw Annex-B or AVCC byte payloads (without start codes or length prefixes).
	NALUs [][]byte
}
