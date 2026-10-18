package util

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// RedactURLError replaces any *url.Error in err's chain with a copy whose URL
// contains only scheme and host (no userinfo, path, or query; port and IPv6
// brackets are kept). The underlying cause remains in the unwrap chain so
// errors.Is and errors.As still reach net.Error timeouts,
// context.DeadlineExceeded, and similar wrapped errors.
func RedactURLError(err error) error {
	if err == nil {
		return nil
	}
	if inners := unwrapErrorSlice(err); inners != nil {
		redacted := make([]error, len(inners))
		changed := false
		for i, inner := range inners {
			next := RedactURLError(inner)
			redacted[i] = next
			if next != inner {
				changed = true
			}
		}
		if changed {
			return errors.Join(redacted...)
		}
		return err
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	replacement := redactSingleURLError(urlErr)
	replaced := replaceURLErrorInChain(err, urlErr, replacement)
	if stillContainsURLSecrets(replaced, urlErr) {
		return fmt.Errorf("network request failed: %w", replacement)
	}
	return replaced
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
	return scheme + "://" + parsed.Host
}

func replaceURLErrorInChain(err, target, replacement error) error {
	if err == nil {
		return nil
	}
	if err == target {
		return replacement
	}
	if inners := unwrapErrorSlice(err); inners != nil {
		next := make([]error, len(inners))
		changed := false
		for i, inner := range inners {
			next[i] = replaceURLErrorInChain(inner, target, replacement)
			if next[i] != inner {
				changed = true
			}
		}
		if changed {
			return errors.Join(next...)
		}
		return err
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

func unwrapErrorSlice(err error) []error {
	type multiUnwrap interface {
		Unwrap() []error
	}
	m, ok := err.(multiUnwrap)
	if !ok {
		return nil
	}
	return m.Unwrap()
}

func stillContainsURLSecrets(err error, urlErr *url.Error) bool {
	if err == nil || urlErr == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, urlErr.URL) {
		return true
	}
	if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
		if parsed.User != nil && strings.Contains(msg, parsed.User.Username()) {
			return true
		}
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(msg, parsed.Path) {
			return true
		}
		if parsed.RawQuery != "" && strings.Contains(msg, parsed.RawQuery) {
			return true
		}
	}
	return false
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
