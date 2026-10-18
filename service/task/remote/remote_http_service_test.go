package remote

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/service/remoteforward"
	"github.com/QueerGlobal/hub-framework/service/remoteforward/remoteforwardtest"
	"github.com/QueerGlobal/hub-framework/util"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func newForwardingServiceForTest(
	t *testing.T,
	host string,
	pathPrefix string,
	settings remoteforward.ForwardSettings,
	backoffConfig util.BackoffConfig,
	clientTimeout time.Duration,
) entity.Task {
	t.Helper()
	settings.UpstreamHost = host
	forwardCfg, err := remoteforward.NewForwardConfig(settings)
	require.NoError(t, err)
	backoffConfig.TargetURL = host
	task := &ForwardingService{
		Host:       host,
		PathPrefix: remoteforward.NormalizePathPrefix(pathPrefix),
		backoff:    util.NewBackoff(backoffConfig),
		httpClient: remoteforward.NewHTTPClient(clientTimeout),
		forward:    forwardCfg,
	}
	t.Cleanup(task.Shutdown)
	return task
}

func newTaskWithSentry(
	t *testing.T,
	host string,
	maxRetries int,
	clientTimeout time.Duration,
	transport *remoteforwardtest.RecordingSentryTransport,
	settings remoteforward.ForwardSettings,
) entity.Task {
	t.Helper()
	settings.Sentry = remoteforward.SentryConfig{
		DSN:       "https://public@sentry.io/1",
		Transport: transport,
	}
	return newForwardingServiceForTest(t, host, "api", settings, testBackoff(maxRetries), clientTimeout)
}

func testRequest(path string) *entity.HTTPServiceRequest {
	return &entity.HTTPServiceRequest{
		Method: entity.HTTPMethodPOST,
		URL:    &url.URL{Path: path},
		Body:   []byte(`{"test":true}`),
	}
}

func TestForwardingService_NameFromConfig(t *testing.T) {
	task, err := NewForwardingService(map[string]interface{}{
		"name":          "remote-forward",
		"host":          "http://127.0.0.1:9",
		"pathprefix":    "api",
		"clientTimeout": float64(1),
		"backoff": map[string]interface{}{
			"initialDelay": float64(0.005),
			"maxDelay":     float64(0.01),
			"multiplier":   float64(1.0),
			"maxRetries":   float64(1),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "remote-forward", task.Name())
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

func TestForwardingService_InvalidForwardHostReportsExactlyOneSentryEvent(t *testing.T) {
	transport := remoteforwardtest.NewRecordingSentryTransport()
	const badHost = "http://forwarduser:forwardpass@%zz"

	task := newTaskWithSentry(t, badHost, 3, time.Second, transport, remoteforward.ForwardSettings{})
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	require.Len(t, transport.Events(), 1, "invalid forward host should report exactly one Sentry event")
	assert.Equal(t, "build_forward_url", transport.Events()[0].Tags["stage"])
}

func TestForwardingService_OversizedResponseReportsExactlyOneSentryEvent(t *testing.T) {
	const limit int64 = 32
	payload := strings.Repeat("w", int(limit+1))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(ts.Close)

	transport := remoteforwardtest.NewRecordingSentryTransport()
	task := newTaskWithSentry(t, ts.URL, 3, time.Second, transport, remoteforward.ForwardSettings{
		MaxResponseBodyBytes: limit,
	})
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	require.Len(t, transport.Events(), 1, "oversized upstream body should report exactly one Sentry event")
	assert.Equal(t, "read_response_body", transport.Events()[0].Tags["stage"])
}

func TestForwardingService_NotFoundReportsNoSentryEvents(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	}))
	t.Cleanup(ts.Close)

	transport := remoteforwardtest.NewRecordingSentryTransport()
	task := newTaskWithSentry(t, ts.URL, 3, time.Second, transport, remoteforward.ForwardSettings{})
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
	assert.Empty(t, transport.Events(), "4xx responses must not emit Sentry events")
}

func TestForwardingService_NetworkTimeoutRedactsURLInErrorLogsAndSentry(t *testing.T) {
	transport := remoteforwardtest.NewRecordingSentryTransport()
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

	task := newTaskWithSentry(t, hostWithCreds, 2, 50*time.Millisecond, transport, remoteforward.ForwardSettings{
		Logger: &log,
	})
	req := testRequest("/secret-path")
	req.URL.RawQuery = "token=SECRET123"

	err = task.Apply(context.Background(), req)
	require.Error(t, err)

	remoteforwardtest.AssertNoForwardURLSecrets(t, err.Error(), logBuf.String())

	events := transport.Events()
	require.Len(t, events, 1, "network give-up should report exactly one Sentry event")
	remoteforwardtest.AssertSentryEventRedacted(t, events[0])
}

func TestForwardingService_ConnectionRefusedRedactsURLInErrorLogsAndSentry(t *testing.T) {
	transport := remoteforwardtest.NewRecordingSentryTransport()
	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf).With().Timestamp().Logger()

	task := newTaskWithSentry(t, "http://forwarduser:forwardpass@127.0.0.1:1", 1, time.Second, transport, remoteforward.ForwardSettings{
		Logger: &log,
	})

	req := testRequest("/secret-path")
	req.URL.RawQuery = "token=SECRET123"

	err := task.Apply(context.Background(), req)
	require.Error(t, err)

	remoteforwardtest.AssertNoForwardURLSecrets(t, err.Error(), logBuf.String())

	events := transport.Events()
	require.Len(t, events, 1, "connection refused give-up should report exactly one Sentry event")
	remoteforwardtest.AssertSentryEventRedacted(t, events[0])
}

func TestForwardingService_RetrySuccessReportsNoSentryEvents(t *testing.T) {
	transport := remoteforwardtest.NewRecordingSentryTransport()
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(ts.Close)

	task := newTaskWithSentry(t, ts.URL, 3, time.Second, transport, remoteforward.ForwardSettings{})
	req := testRequest("/v1/resource")
	require.NoError(t, task.Apply(context.Background(), req))
	assert.Equal(t, int32(2), calls.Load())
	assert.Empty(t, transport.Events(), "retries that eventually succeed must not emit Sentry events")
}

func TestForwardingService_ClientTimeoutReportsExactlyOneSentryEvent(t *testing.T) {
	transport := remoteforwardtest.NewRecordingSentryTransport()
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	task := newTaskWithSentry(t, ts.URL, 2, 50*time.Millisecond, transport, remoteforward.ForwardSettings{})
	err := task.Apply(context.Background(), testRequest("/v1/resource"))
	require.Error(t, err)
	assert.Equal(t, int32(2), calls.Load())
	require.Len(t, transport.Events(), 1, "timeout give-up after retries should report exactly one Sentry event")
	remoteforwardtest.AssertSentryEventRedacted(t, transport.Events()[0])
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
