package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/util"
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
		"host":        tsURL,
		"pathprefix":  "api",
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
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<html>error</html>`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 2, 1)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, 2, calls)
	assert.NotContains(t, err.Error(), util.UnrecoverableErrorMsg)
}

func TestForwardingService_ClientErrorNoRetry(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 3, 1)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
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
		"host":                   ts.URL,
		"pathprefix":             "api",
		"clientTimeout":          float64(1),
		"maxResponseBodyBytes":   float64(limit),
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

func TestForwardingService_ClientTimeout(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(ts.Close)

	task := newTask(t, ts.URL, 2, 0.05)
	req := testRequest("/v1/resource")

	err := task.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, 2, calls)
}
