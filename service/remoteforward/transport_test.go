package remoteforward

import (
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

// recordingTransport captures Sentry events for tests.
type recordingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *recordingTransport) Flush(time.Duration) bool { return true }

func (t *recordingTransport) Configure(sentry.ClientOptions) {}

func (t *recordingTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *recordingTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentry.Event, len(t.events))
	copy(out, t.events)
	return out
}
