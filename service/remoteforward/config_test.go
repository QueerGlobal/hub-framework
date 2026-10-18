package remoteforward

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewForwardConfigResilient_InvalidSentryDSN(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf).With().Timestamp().Logger()

	cfg := NewForwardConfigResilient(ForwardSettings{
		Logger:       &log,
		UpstreamHost: "http://upstream.example",
		Sentry:       SentryConfig{DSN: "not-a-valid-sentry-dsn"},
	})

	_, ok := cfg.reporter.(noopErrorReporter)
	assert.True(t, ok)
	assert.Contains(t, buf.String(), "Sentry init failed")
	assert.Contains(t, buf.String(), "continuing without error reporting")
}

func TestNewForwardConfigResilient_ValidWithoutSentry(t *testing.T) {
	cfg := NewForwardConfigResilient(ForwardSettings{
		UpstreamHost: "http://upstream.example",
	})
	require.NotNil(t, cfg.reporter)
}
