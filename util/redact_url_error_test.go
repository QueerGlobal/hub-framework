package util

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactURLError_StripsPathQueryAndUserinfo(t *testing.T) {
	inner := context.DeadlineExceeded
	rawURL := "http://user:password@upstream.example/secret-path?token=SECRET123"
	urlErr := &url.Error{Op: "Get", URL: rawURL, Err: inner}
	wrapped := fmt.Errorf("failed to perform HTTP request: %w", urlErr)

	redacted := RedactURLError(wrapped)
	require.NotNil(t, redacted)

	out := redacted.Error()
	assert.Contains(t, out, "http://upstream.example")
	assert.NotContains(t, out, "SECRET123")
	assert.NotContains(t, out, "secret-path")
	assert.NotContains(t, out, "user")
	assert.NotContains(t, out, "password")
}

func TestRedactURLError_PreservesTimeoutDetection(t *testing.T) {
	inner := &net.OpError{
		Op:  "dial",
		Err: &timeoutErr{},
	}
	urlErr := &url.Error{
		Op:  "Get",
		URL: "http://user:pw@host.example/path?q=1",
		Err: inner,
	}

	redacted := RedactURLError(urlErr)
	require.NotNil(t, redacted)

	var timeout interface{ Timeout() bool }
	require.True(t, errors.As(redacted, &timeout))
	assert.True(t, timeout.Timeout())
	assert.True(t, errors.Is(redacted, inner))
}

func TestRedactURLError_PreservesContextDeadlineExceeded(t *testing.T) {
	urlErr := &url.Error{
		Op:  "Post",
		URL: "http://user:pw@host.example/secret?token=x",
		Err: context.DeadlineExceeded,
	}
	redacted := RedactURLError(fmt.Errorf("outer: %w", urlErr))
	assert.True(t, errors.Is(redacted, context.DeadlineExceeded))
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestRedactURLError_NilSafe(t *testing.T) {
	assert.Nil(t, RedactURLError(nil))
}

func TestRedactURLError_PassthroughNonURL(t *testing.T) {
	err := fmt.Errorf("plain error")
	assert.Equal(t, err, RedactURLError(err))
}

func TestRedactedURLFromRaw_SchemelessHost(t *testing.T) {
	got := redactedURLFromRaw("//example.com/path?x=1")
	assert.Equal(t, "http://example.com", got)
}
