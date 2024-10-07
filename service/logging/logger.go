package logging

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
)

var (
	instance zerolog.Logger
	once     sync.Once
)

func GetLogger() *zerolog.Logger {
	once.Do(func() {
		instance = zerolog.New(os.Stdout).With().Timestamp().Logger()
	})
	return &instance
}

func SetLogLevel(level zerolog.Level) {
	instance = instance.Level(level)
}
