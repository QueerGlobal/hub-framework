package util

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// RedactURLError replaces any *url.Error in err's chain with a copy whose URL
// contains only scheme and hostname (no userinfo, path, or query). The
// underlying cause remains in the unwrap chain so errors.Is and errors.As still
// reach net.Error timeouts, context.DeadlineExceeded, and similar wrapped errors.
func RedactURLError(err error) error {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	replacement := redactSingleURLError(urlErr)
	return replaceURLErrorInChain(err, urlErr, replacement)
}

type redactedURLError struct {
	op  string
	url string
	err error
}

func (e *redactedURLError) Error() string {
	return fmt.Sprintf("%s %q: %v", e.op, e.url, e.err)
}

func (e *redactedURLError) Unwrap() error {
	return e.err
}

func (e *redactedURLError) Timeout() bool {
	return timeoutError(e.err)
}

func (e *redactedURLError) Temporary() bool {
	return temporaryError(e.err)
}

func redactSingleURLError(u *url.Error) *redactedURLError {
	inner := u.Err
	if inner != nil {
		inner = RedactURLError(inner)
	}
	return &redactedURLError{
		op:  u.Op,
		url: redactedURLFromRaw(u.URL),
		err: inner,
	}
}

func redactedURLFromRaw(raw string) string {
	if raw == "" {
		return ""
	}
	toParse := raw
	if strings.HasPrefix(raw, "//") {
		toParse = "http:" + raw
	}
	parsed, err := url.Parse(toParse)
	if err != nil || parsed.Host == "" {
		return "<redacted-url>"
	}
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + parsed.Hostname()
}

func replaceURLErrorInChain(err, target, replacement error) error {
	if err == nil {
		return nil
	}
	if err == target {
		return replacement
	}
	inner := errors.Unwrap(err)
	if inner == nil {
		return err
	}
	newInner := replaceURLErrorInChain(inner, target, replacement)
	if newInner == inner {
		return err
	}
	prefix := errorPrefix(err, inner)
	if prefix == "" {
		return newInner
	}
	return fmt.Errorf("%s: %w", prefix, newInner)
}

func errorPrefix(outer, inner error) string {
	outerMsg := outer.Error()
	innerMsg := inner.Error()
	if strings.HasSuffix(outerMsg, innerMsg) {
		rest := strings.TrimSuffix(outerMsg, innerMsg)
		return strings.TrimSuffix(rest, ": ")
	}
	return ""
}

func timeoutError(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	t, ok := err.(timeout)
	return ok && t.Timeout()
}

func temporaryError(err error) bool {
	type temporary interface {
		Temporary() bool
	}
	t, ok := err.(temporary)
	return ok && t.Temporary()
}
