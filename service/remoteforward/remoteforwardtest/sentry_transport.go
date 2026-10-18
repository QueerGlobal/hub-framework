package remoteforwardtest

import (
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RecordingSentryTransport captures Sentry events in tests.
type RecordingSentryTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func NewRecordingSentryTransport() *RecordingSentryTransport {
	return &RecordingSentryTransport{}
}

func (t *RecordingSentryTransport) Flush(time.Duration) bool { return true }

func (t *RecordingSentryTransport) Configure(sentry.ClientOptions) {}

func (t *RecordingSentryTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *RecordingSentryTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentry.Event, len(t.events))
	copy(out, t.events)
	return out
}

func AssertNoForwardURLSecrets(t *testing.T, parts ...string) {
	t.Helper()
	for _, part := range parts {
		assert.NotContains(t, part, "SECRET123")
		assert.NotContains(t, part, "secret-path")
		assert.NotContains(t, part, "forwarduser")
		assert.NotContains(t, part, "forwardpass")
	}
}

func AssertSentryEventRedacted(t *testing.T, event *sentry.Event) {
	t.Helper()
	require.NotEmpty(t, event.Exception)
	for _, exc := range event.Exception {
		AssertNoForwardURLSecrets(t, exc.Value)
	}
}
