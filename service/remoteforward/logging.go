package remoteforward

import (
	"os"

	"github.com/rs/zerolog"
)

// DefaultLogger returns a JSON structured logger writing to stderr.
func DefaultLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).With().Timestamp().Logger()
}
