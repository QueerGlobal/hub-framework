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

	cfg, err := NewForwardConfigResilient(ForwardSettings{
		Logger:       &log,
		UpstreamHost: "http://upstream.example",
		Sentry:       SentryConfig{DSN: "not-a-valid-sentry-dsn"},
	})
	require.NoError(t, err)

	_, ok := cfg.reporter.(noopErrorReporter)
	assert.True(t, ok)
	assert.Contains(t, buf.String(), "Sentry init failed")
	assert.Contains(t, buf.String(), "continuing without error reporting")
}

func TestNewForwardConfigResilient_ValidWithoutSentry(t *testing.T) {
	cfg, err := NewForwardConfigResilient(ForwardSettings{
		UpstreamHost: "http://upstream.example",
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.reporter)
}
