package buffer

import (
	"time"
)

// Frame представляет один видеокадр (Access Unit), состоящий из одной или нескольких NALU.
type Frame struct {
	Timestamp  time.Duration
	IsKeyFrame bool
	NALUs      [][]byte
}
