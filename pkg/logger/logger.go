package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func New(appEnv, level string) (zerolog.Logger, error) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if strings.EqualFold(appEnv, "development") {
		log = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}
	lvl, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	return log, nil
}
