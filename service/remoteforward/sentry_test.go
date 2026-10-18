package remoteforward

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSentryReporter_WithDSNCreatesClient(t *testing.T) {
	transport := &recordingTransport{}
	reporter, shutdown, err := newSentryReporter(SentryConfig{
		DSN:         "https://public@sentry.io/1",
		Environment: "test",
		Release:     "hub-framework@dev",
		SampleRate:  0.5,
	}, transport)
	require.NoError(t, err)
	defer shutdown()

	_, ok := reporter.(*sentryReporter)
	assert.True(t, ok)
}
