package remote

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/util"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingSentryTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *recordingSentryTransport) Flush(time.Duration) bool { return true }

func (t *recordingSentryTransport) Configure(sentry.ClientOptions) {}

func (t *recordingSentryTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *recordingSentryTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentry.Event, len(t.events))
	copy(out, t.events)
	return out
}

func assertNoForwardURLSecrets(t *testing.T, parts ...string) {
	t.Helper()
	for _, part := range parts {
		assert.NotContains(t, part, "SECRET123")
		assert.NotContains(t, part, "secret-path")
		assert.NotContains(t, part, "forwarduser")
		assert.NotContains(t, part, "forwardpass")
	}
}

func assertSentryEventRedacted(t *testing.T, event *sentry.Event) {
	t.Helper()
	require.NotEmpty(t, event.Exception)
	for _, exc := range event.Exception {
		assertNoForwardURLSecrets(t, exc.Value)
	}
}

func testBackoff(maxRetries int) util.BackoffConfig {
	return util.BackoffConfig{
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   1.0,
		MaxRetries:   maxRetries,
	}
}

func newTask(t *testing.T, tsURL string, maxRetries int, clientTimeoutSec float64) entity.Task {
	t.Helper()
	task, err := NewForwardingService(map[string]interface{}{
		"host":          tsURL,
		"pathprefix":    "api",
		"clientTimeout": clientTimeoutSec,
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(maxRetries),
		},
	})
	require.NoError(t, err)
	return task
}

func testRequest(path string) *entity.HTTPServiceRequest {
	return &entity.HTTPServiceRequest{
		Method: entity.HTTPMethodPOST,
		URL:    &url.URL{Path: path},
		Body:   []byte(`{"test":true}`),
	}
}

func TestForwardingService_SuccessJSONResponse(t *testing.T) {
	const wantBody = `{"result":"ok"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(wantBody))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 2, 1)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.NoError(t, err)

	resp := req.GetResponse()
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.GetResponseMeta().GetStatusCode())
	assert.JSONEq(t, wantBody, string(resp.GetBody()))
}

func TestForwardingService_ServerErrorRetriesThenFails(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<html>error</html>`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 2, 1)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, int32(2), calls.Load())
	assert.NotContains(t, err.Error(), util.UnrecoverableErrorMsg)
}

func TestForwardingService_ClientErrorNoRetry(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 3, 1)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
	assert.Contains(t, err.Error(), util.UnrecoverableErrorMsg)
}

func TestForwardingService_NoDoubleSlashInForwardedPath(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 1, 1)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/resource", seenPath)
	assert.False(t, strings.Contains(seenPath, "//"))
}

func TestForwardingService_ResponseBodyUnderLimit(t *testing.T) {
	const limit int64 = 200
	payload := strings.Repeat("z", int(limit-1))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(ts.Close)

	task, err := NewForwardingService(map[string]interface{}{
		"host":                 ts.URL,
		"pathprefix":           "api",
		"clientTimeout":        float64(1),
		"maxResponseBodyBytes": float64(limit),
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(1),
		},
	})
	require.NoError(t, err)
	req := testRequest("/v1/resource")

	err = task.Apply(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, req.GetResponse().GetBody(), int(limit-1))
}

func TestForwardingService_ResponseBodyOverLimit(t *testing.T) {
	const limit int64 = 32
	payload := strings.Repeat("w", int(limit+1))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(ts.Close)

	task, err := NewForwardingService(map[string]interface{}{
		"host":                 ts.URL,
		"pathprefix":           "api",
		"clientTimeout":        float64(1),
		"maxResponseBodyBytes": float64(limit),
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(1),
		},
	})
	require.NoError(t, err)
	req := testRequest("/v1/resource")

	err = task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func TestNewForwardingService_InvalidSentryDSNStillWorks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(ts.Close)

	task, err := NewForwardingService(map[string]interface{}{
		"host":          ts.URL,
		"pathprefix":    "api",
		"clientTimeout": float64(1),
		"sentry": map[string]interface{}{
			"dsn": "not-a-valid-sentry-dsn",
		},
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(1),
		},
	})
	require.NoError(t, err)
	req := testRequest("/v1/resource")
	require.NoError(t, task.Apply(context.Background(), req))
}

func TestForwardingService_NetworkTimeoutRedactsURLInErrorLogsAndSentry(t *testing.T) {
	transport := &recordingSentryTransport{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	parsed, err := url.Parse(ts.URL)
	require.NoError(t, err)
	hostWithCreds := fmt.Sprintf("http://forwarduser:forwardpass@%s", parsed.Host)

	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf).With().Timestamp().Logger()

	task, err := NewForwardingService(map[string]interface{}{
		"host":            hostWithCreds,
		"pathprefix":      "api",
		"clientTimeout":   float64(0.05),
		"logger":          &log,
		"sentryTransport": transport,
		"sentry": map[string]interface{}{
			"dsn": "https://public@sentry.io/1",
		},
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(2),
		},
	})
	require.NoError(t, err)
	if shutdown, ok := task.(interface{ Shutdown() }); ok {
		t.Cleanup(shutdown.Shutdown)
	}

	req := testRequest("/secret-path")
	req.URL.RawQuery = "token=SECRET123"

	err = task.Apply(context.Background(), req)
	require.Error(t, err)

	assertNoForwardURLSecrets(t, err.Error(), logBuf.String())

	events := transport.Events()
	require.NotEmpty(t, events)
	assertSentryEventRedacted(t, events[len(events)-1])
}

func TestForwardingService_ConnectionRefusedRedactsURLInErrorLogsAndSentry(t *testing.T) {
	transport := &recordingSentryTransport{}
	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf).With().Timestamp().Logger()

	task, err := NewForwardingService(map[string]interface{}{
		"host":            "http://forwarduser:forwardpass@127.0.0.1:1",
		"pathprefix":      "api",
		"clientTimeout":   float64(1),
		"logger":          &log,
		"sentryTransport": transport,
		"sentry": map[string]interface{}{
			"dsn": "https://public@sentry.io/1",
		},
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(1),
		},
	})
	require.NoError(t, err)
	if shutdown, ok := task.(interface{ Shutdown() }); ok {
		t.Cleanup(shutdown.Shutdown)
	}

	req := testRequest("/secret-path")
	req.URL.RawQuery = "token=SECRET123"

	err = task.Apply(context.Background(), req)
	require.Error(t, err)

	assertNoForwardURLSecrets(t, err.Error(), logBuf.String())

	events := transport.Events()
	require.NotEmpty(t, events)
	assertSentryEventRedacted(t, events[0])
}

func TestForwardingService_ClientTimeout(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 2, 0.05)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, int32(2), calls.Load())
}
