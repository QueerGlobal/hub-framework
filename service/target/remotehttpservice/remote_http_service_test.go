package remotehttpservice

import (
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
	assert.Equal(t, http.StatusOK, resp.GetResponseMeta().GetStatusCode())
	assert.JSONEq(t, wantBody, string(resp.GetBody()))
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
	payload := strings.Repeat("x", int(limit-1))
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
	assert.Len(t, resp.GetBody(), int(limit-1))
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
