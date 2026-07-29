package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init настраивает глобальный логгер zerolog.
func Init(debug bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	zerolog.SetGlobalLevel(level)

	// Используем консольный вывод для удобства разработки
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
}
