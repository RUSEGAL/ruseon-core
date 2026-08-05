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
	
	// По умолчанию все логи из стандартной библиотеки будем считать предупреждениями (т.к. обычно это ошибки декодера/потеря пакетов)
	log.Warn().Str("source", "stdlog").Msg(msg)
	return len(p), nil
}

// Init настраивает глобальный логгер zerolog.
func Init(debug bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	zerolog.SetGlobalLevel(level)

	// Используем консольный вывод для удобства разработки и бродкастер для SSE
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	multi := zerolog.MultiLevelWriter(consoleWriter, GlobalBroadcaster)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	// Перехватываем стандартный логгер Go (который использует gortsplib для вывода RTP packet lost)
	stdlog.SetFlags(0)
	stdlog.SetOutput(&stdLogAdapter{})
}
