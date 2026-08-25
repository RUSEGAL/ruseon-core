// Package logger configures high-performance structured logging using zerolog,
// integrates real-time Server-Sent Events (SSE) log streaming, and intercepts standard
// library logging outputs.
package logger

import (
	stdlog "log"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type stdLogAdapter struct{}

func (a *stdLogAdapter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	
	// Treat third-party library standard logging output as warning level (e.g. gortsplib RTP packet loss)
	log.Warn().Str("source", "stdlog").Msg(msg)
	return len(p), nil
}

// Init configures the global zerolog logger and standard library logging output.
//
// When debug is true, log level is set to DebugLevel; otherwise InfoLevel.
// Outputs are multiplexed simultaneously to stdout (via ConsoleWriter) and to
// GlobalBroadcaster for live web UI streaming over Server-Sent Events (/api/logs/stream).
func Init(debug bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	zerolog.SetGlobalLevel(level)

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	multi := zerolog.MultiLevelWriter(consoleWriter, GlobalBroadcaster)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	// Redirect standard library logger output through zerolog
	stdlog.SetFlags(0)
	stdlog.SetOutput(&stdLogAdapter{})
}
