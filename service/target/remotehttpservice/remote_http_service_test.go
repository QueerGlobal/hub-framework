package remotehttpservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/service/remoteforward"
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
	}
}

func assertTimeoutDetectable(t *testing.T, err error) {
	t.Helper()
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return
	}
	assert.Contains(t, strings.ToLower(err.Error()), "timeout")
}

func testBackoff(maxRetries int) util.BackoffConfig {
	return util.BackoffConfig{
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   1.0,
		MaxRetries:   maxRetries,
	}
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

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          ts.URL,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(2),
		ClientTimeout: time.Second,
	})

	resp, err := svc.Apply(testRequest("/v1/resource"))
	require.NoError(t, err)
	require.NotNil(t, resp)

	encoded, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.JSONEq(t, wantBody, string(encoded))
	assert.Equal(t, http.StatusOK, resp.GetResponseMeta().GetStatusCode())
}

func TestNewForwardingService_InvalidSentryDSNStillWorks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(ts.Close)

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          ts.URL,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(1),
		ClientTimeout: time.Second,
		Sentry:        remoteforward.SentryConfig{DSN: "not-a-valid-sentry-dsn"},
	})

	resp, err := svc.Apply(testRequest("/v1/resource"))
	require.NoError(t, err)
	encoded, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(encoded))
}

func TestForwardingService_ServerErrorRetriesThenFails(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(ts.Close)

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          ts.URL,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(2),
		ClientTimeout: time.Second,
	})

	_, err := svc.Apply(testRequest("/v1/resource"))
	require.Error(t, err)
	assert.Equal(t, 2, calls)
	assert.NotContains(t, err.Error(), util.UnrecoverableErrorMsg)
}

func TestForwardingService_ClientErrorNoRetry(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad input"}`))
	}))
	t.Cleanup(ts.Close)

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          ts.URL,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(3),
		ClientTimeout: time.Second,
	})

	_, err := svc.Apply(testRequest("/v1/resource"))
	require.Error(t, err)
	assert.Equal(t, 1, calls)
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

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          ts.URL,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(1),
		ClientTimeout: time.Second,
	})

	_, err := svc.Apply(testRequest("/v1/resource"))
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/resource", seenPath)
	assert.False(t, strings.Contains(seenPath, "//"))
}

func TestForwardingService_ResponseBodyUnderLimit(t *testing.T) {
	const limit int64 = 256
	payloadField := strings.Repeat("x", int(limit-20))
	payload := `{"data":"` + payloadField + `"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(ts.Close)

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:                 ts.URL,
		PathPrefix:           "api",
		BackoffConfig:        testBackoff(1),
		ClientTimeout:        time.Second,
		MaxResponseBodyBytes: limit,
	})

	resp, err := svc.Apply(testRequest("/v1/resource"))
	require.NoError(t, err)
	assert.LessOrEqual(t, int64(len(resp.GetBody())), limit)
}

func TestForwardingService_ResponseBodyOverLimit(t *testing.T) {
	const limit int64 = 64
	payload := strings.Repeat("y", int(limit+1))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(ts.Close)

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:                 ts.URL,
		PathPrefix:           "api",
		BackoffConfig:        testBackoff(1),
		ClientTimeout:        time.Second,
		MaxResponseBodyBytes: limit,
	})

	_, err := svc.Apply(testRequest("/v1/resource"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
	assert.Contains(t, err.Error(), util.UnrecoverableErrorMsg)
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

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          hostWithCreds,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(2),
		ClientTimeout: 50 * time.Millisecond,
		Logger:        &log,
		Sentry: remoteforward.SentryConfig{
			DSN:       "https://public@sentry.io/1",
			Transport: transport,
		},
	})
	t.Cleanup(svc.Shutdown)

	req := testRequest("/secret-path")
	req.URL.RawQuery = "token=SECRET123"

	_, err = svc.Apply(req)
	require.Error(t, err)

	assertNoForwardURLSecrets(t, err.Error(), logBuf.String())
	assertTimeoutDetectable(t, err)

	events := transport.Events()
	require.NotEmpty(t, events)
	require.NotEmpty(t, events[len(events)-1].Exception)
	sentryText := events[len(events)-1].Exception[0].Value
	assertNoForwardURLSecrets(t, sentryText)
	assert.Equal(t, "http_request", events[len(events)-1].Tags["stage"])
}

func TestForwardingService_ConnectionRefusedRedactsURLInErrorLogsAndSentry(t *testing.T) {
	transport := &recordingSentryTransport{}
	hostWithCreds := "http://forwarduser:forwardpass@127.0.0.1:1"

	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf).With().Timestamp().Logger()

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          hostWithCreds,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(1),
		ClientTimeout: time.Second,
		Logger:        &log,
		Sentry: remoteforward.SentryConfig{
			DSN:       "https://public@sentry.io/1",
			Transport: transport,
		},
	})
	t.Cleanup(svc.Shutdown)

	req := testRequest("/secret-path")
	req.URL.RawQuery = "token=SECRET123"

	_, err := svc.Apply(req)
	require.Error(t, err)

	assertNoForwardURLSecrets(t, err.Error(), logBuf.String())

	events := transport.Events()
	require.NotEmpty(t, events)
	require.NotEmpty(t, events[0].Exception)
	assertNoForwardURLSecrets(t, events[0].Exception[0].Value)
}

func TestForwardingService_ClientTimeout(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(ts.Close)

	svc := NewForwardingService(ForwardingServiceConfig{
		Host:          ts.URL,
		PathPrefix:    "api",
		BackoffConfig: testBackoff(2),
		ClientTimeout: 50 * time.Millisecond,
	})

	_, err := svc.Apply(testRequest("/v1/resource"))
	require.Error(t, err)
	assert.Equal(t, 2, calls)
}
