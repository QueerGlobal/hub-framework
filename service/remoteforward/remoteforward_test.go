package remoteforward

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/QueerGlobal/hub-framework/util"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
)

func testForwardConfig(t *testing.T, mutate ...func(*ForwardSettings)) ForwardConfig {
	t.Helper()
	settings := ForwardSettings{UpstreamHost: "http://upstream.example"}
	for _, fn := range mutate {
		fn(&settings)
	}
	cfg, err := NewForwardConfig(settings)
	require.NoError(t, err)
	return cfg
}

func TestBuildForwardURL_QueryString(t *testing.T) {
	reqURL := &url.URL{Path: "/v1/resource", RawQuery: "q=hello&x=1"}
	got, err := BuildForwardURL("http://upstream.example", "/api/", reqURL, testForwardConfig(t))
	require.NoError(t, err)
	assert.Equal(t, "http://upstream.example/api/v1/resource?q=hello&x=1", got)
}

func TestBuildForwardURL_TrailingSlashOnHost(t *testing.T) {
	reqURL := &url.URL{Path: "/v1/resource"}
	got, err := BuildForwardURL("http://upstream.example/", "/api", reqURL, testForwardConfig(t))
	require.NoError(t, err)
	assert.Equal(t, "http://upstream.example/api/v1/resource", got)
}

func TestBuildForwardURL_EncodedPath(t *testing.T) {
	reqURL, err := url.Parse("/v1/foo%2Fbar")
	require.NoError(t, err)
	got, err := BuildForwardURL("http://upstream.example", "/api/", reqURL, testForwardConfig(t))
	require.NoError(t, err)
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/foo/bar", parsed.Path)
}

func TestBuildForwardURL_HostEscapeAttempt(t *testing.T) {
	reqURL := &url.URL{Path: "//evil.example/steal"}
	_, err := BuildForwardURL("http://upstream.example", "/api/", reqURL, testForwardConfig(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol-relative")
}

func TestBuildForwardURL_AbsoluteURLEscapeAttempt(t *testing.T) {
	reqURL, err := url.Parse("http://evil.example/steal")
	require.NoError(t, err)
	_, err = BuildForwardURL("http://upstream.example", "/api/", reqURL, testForwardConfig(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escape target host")
}

func TestReadLimitedBody_UnderLimit(t *testing.T) {
	const limit int64 = 128
	payload := bytes.Repeat([]byte("a"), int(limit-1))
	body, err := ReadLimitedBody(bytes.NewReader(payload), limit, testForwardConfig(t))
	require.NoError(t, err)
	assert.Len(t, body, int(limit-1))
}

func TestReadLimitedBody_OverLimit(t *testing.T) {
	const limit int64 = 64
	payload := bytes.Repeat([]byte("b"), int(limit+1))
	_, err := ReadLimitedBody(bytes.NewReader(payload), limit, testForwardConfig(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), util.UnrecoverableErrorMsg)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func TestNewSentryReporter_NoDSNIsNoop(t *testing.T) {
	r, shutdown, err := NewSentryReporter(SentryConfig{})
	require.NoError(t, err)
	defer shutdown()
	_, ok := r.(noopErrorReporter)
	assert.True(t, ok)
}

func TestReadLimitedBody_DefaultLimitWhenZero(t *testing.T) {
	small := io.NopCloser(strings.NewReader("ok"))
	cfg := testForwardConfig(t, func(s *ForwardSettings) {
		s.MaxResponseBodyBytes = 8
	})
	_, err := ReadLimitedBody(small, 0, cfg)
	require.NoError(t, err)
}

func TestNewForwardConfig_DefaultLoggerLogsLimitExceeded(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf).With().Timestamp().Logger()
	cfg, err := NewForwardConfig(ForwardSettings{
		Logger:       &log,
		UpstreamHost: "http://upstream.example",
	})
	require.NoError(t, err)

	_, err = ReadLimitedBody(bytes.NewReader(bytes.Repeat([]byte("z"), 65)), 64, cfg)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "upstream response body exceeded configured limit")
	assert.Contains(t, buf.String(), "target_host")
	assert.Contains(t, buf.String(), "upstream.example")
}

func TestSentryReporter_InvalidForwardURLNoSecretsOrQuery(t *testing.T) {
	transport := &recordingTransport{}
	reporter, shutdown, err := newSentryReporter(SentryConfig{
		DSN: "https://public@sentry.io/1",
	}, transport)
	require.NoError(t, err)
	defer shutdown()

	cfg := ForwardConfig{
		logger:        DefaultLogger(),
		reporter:      reporter,
		targetHostTag: TargetHostTag("http://apiuser:topsecret@upstream.example:8080"),
	}

	reqURL, err := url.Parse("http://evil.example/steal?token=leaked&x=1")
	require.NoError(t, err)
	_, err = BuildForwardURL("http://apiuser:topsecret@upstream.example:8080", "/api/", reqURL, cfg)
	require.Error(t, err)

	events := transport.Events()
	require.NotEmpty(t, events)
	event := events[0]
	assert.Equal(t, "upstream.example:8080", event.Tags["target_host"])
	assert.Equal(t, "build_forward_url", event.Tags["stage"])
	require.NotEmpty(t, event.Exception)
	assert.NotContains(t, event.Exception[0].Value, "token=leaked")
	assert.NotContains(t, event.Exception[0].Value, "topsecret")
	payload, err := json.Marshal(event.Tags)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "token=leaked")
}

func TestSentryReporter_BodyLimitReportsTargetHostOnly(t *testing.T) {
	transport := &recordingTransport{}
	reporter, shutdown, err := newSentryReporter(SentryConfig{
		DSN: "https://public@sentry.io/1",
	}, transport)
	require.NoError(t, err)
	defer shutdown()

	cfg := ForwardConfig{
		logger:        DefaultLogger(),
		reporter:      reporter,
		targetHostTag: "upstream.example",
	}

	_, err = ReadLimitedBody(bytes.NewReader(bytes.Repeat([]byte("x"), 10)), 4, cfg)
	require.Error(t, err)

	events := transport.Events()
	require.NotEmpty(t, events)
	event := events[0]
	assert.Equal(t, "read_response_body", event.Tags["stage"])
	assert.Equal(t, "upstream.example", event.Tags["target_host"])
}

func TestBuildForwardURL_InvalidHostNoCredentialLeak(t *testing.T) {
	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf).With().Timestamp().Logger()
	transport := &recordingTransport{}
	reporter, shutdown, err := newSentryReporter(SentryConfig{DSN: "https://public@sentry.io/1"}, transport)
	require.NoError(t, err)
	defer shutdown()

	cfg := ForwardConfig{
		logger:   log,
		reporter: reporter,
	}

	const host = "http://user:secret@%zz"
	_, err = BuildForwardURL(host, "/api/", nil, cfg)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
	assert.NotContains(t, err.Error(), "user:")
	assert.NotContains(t, logBuf.String(), "secret")

	events := transport.Events()
	require.NotEmpty(t, events)
	event := events[0]
	require.NotEmpty(t, event.Exception)
	assert.NotContains(t, event.Exception[0].Value, "secret")
	assert.Equal(t, "build_forward_url", event.Tags["stage"])
}

func TestTargetHostTag_StripsCredentials(t *testing.T) {
	assert.Equal(t, "upstream.example", TargetHostTag("http://user:pass@upstream.example/path?x=1"))
	assert.Equal(t, "[::1]:8080", TargetHostTag("http://user:pass@[::1]:8080/path?x=1"))
}
