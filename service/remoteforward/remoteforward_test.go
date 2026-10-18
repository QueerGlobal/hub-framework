package remoteforward

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/QueerGlobal/hub-framework/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
)

func TestBuildForwardURL_QueryString(t *testing.T) {
	reqURL := &url.URL{Path: "/v1/resource", RawQuery: "q=hello&x=1"}
	got, err := BuildForwardURL("http://upstream.example", "/api/", reqURL, Options{})
	require.NoError(t, err)
	assert.Equal(t, "http://upstream.example/api/v1/resource?q=hello&x=1", got)
}

func TestBuildForwardURL_TrailingSlashOnHost(t *testing.T) {
	reqURL := &url.URL{Path: "/v1/resource"}
	got, err := BuildForwardURL("http://upstream.example/", "/api", reqURL, Options{})
	require.NoError(t, err)
	assert.Equal(t, "http://upstream.example/api/v1/resource", got)
}

func TestBuildForwardURL_EncodedPath(t *testing.T) {
	reqURL, err := url.Parse("/v1/foo%2Fbar")
	require.NoError(t, err)
	got, err := BuildForwardURL("http://upstream.example", "/api/", reqURL, Options{})
	require.NoError(t, err)
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/foo/bar", parsed.Path)
}

func TestBuildForwardURL_HostEscapeAttempt(t *testing.T) {
	reqURL := &url.URL{Path: "//evil.example/steal"}
	_, err := BuildForwardURL("http://upstream.example", "/api/", reqURL, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol-relative")
}

func TestBuildForwardURL_AbsoluteURLEscapeAttempt(t *testing.T) {
	reqURL, err := url.Parse("http://evil.example/steal")
	require.NoError(t, err)
	_, err = BuildForwardURL("http://upstream.example", "/api/", reqURL, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escape target host")
}

func TestReadLimitedBody_UnderLimit(t *testing.T) {
	const limit int64 = 128
	payload := bytes.Repeat([]byte("a"), int(limit-1))
	body, err := ReadLimitedBody(bytes.NewReader(payload), limit, Options{})
	require.NoError(t, err)
	assert.Len(t, body, int(limit-1))
}

func TestReadLimitedBody_OverLimit(t *testing.T) {
	const limit int64 = 64
	payload := bytes.Repeat([]byte("b"), int(limit+1))
	_, err := ReadLimitedBody(bytes.NewReader(payload), limit, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), util.UnrecoverableErrorMsg)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func TestNewErrorReporter_NoDSNIsNoop(t *testing.T) {
	r := NewErrorReporter(SentryConfig{})
	_, ok := r.(noopErrorReporter)
	assert.True(t, ok)
}

type recordingReporter struct {
	called bool
}

func (r *recordingReporter) ReportError(err error, tags map[string]string) {
	r.called = true
}

func TestNewErrorReporter_WithDSNUsesHook(t *testing.T) {
	r := NewErrorReporter(SentryConfig{DSN: "https://example@sentry.io/1"})
	rep, ok := r.(*dsnErrorReporter)
	require.True(t, ok)
	assert.Equal(t, "https://example@sentry.io/1", rep.dsn)
}

func TestReadLimitedBody_DefaultLimitWhenZero(t *testing.T) {
	small := io.NopCloser(strings.NewReader("ok"))
	_, err := ReadLimitedBody(small, 0, Options{MaxResponseBodyBytes: 8})
	require.NoError(t, err)
}
