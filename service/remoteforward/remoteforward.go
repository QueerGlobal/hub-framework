// Package remoteforward provides shared HTTP forwarding utilities for remote target
// services and workflow tasks: safe URL resolution with net/url, bounded upstream
// response reads, structured logging (zerolog), and optional Sentry error reporting.
package remoteforward

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/util"
)

const (
	DefaultClientTimeout        = 30 * time.Second
	DefaultMaxResponseBodyBytes = 10 << 20 // 10 MiB
)

func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}
	return &http.Client{Timeout: timeout}
}

// BuildForwardURL resolves host, path prefix, and request URL into a single forward target.
// The result always shares the host and scheme of host; request paths cannot escape to another host.
func BuildForwardURL(host, pathPrefix string, requestURL *url.URL, cfg ForwardConfig) (string, error) {
	base, err := url.Parse(host)
	if err != nil {
		return "", cfg.failInvalidForwardURL(invalidForwardHostError(host))
	}
	if base.Scheme == "" || base.Host == "" {
		return "", cfg.failInvalidForwardURL(fmt.Errorf("invalid forward host %q: scheme and host are required", redactedHostURL(host)))
	}

	if err := validateRequestURLForForward(base, requestURL); err != nil {
		return "", cfg.failInvalidForwardURL(err)
	}

	joinedPath, err := joinForwardPath(pathPrefix, requestURL)
	if err != nil {
		return "", cfg.failInvalidForwardURL(err)
	}

	resolved, err := resolveForwardReference(base, requestURL, joinedPath)
	if err != nil {
		return "", cfg.failInvalidForwardURL(err)
	}

	return resolved.String(), nil
}

func validateRequestURLForForward(base *url.URL, requestURL *url.URL) error {
	if requestURL == nil {
		return nil
	}
	if requestURL.IsAbs() && requestURL.Host != "" && requestURL.Host != base.Host {
		return fmt.Errorf("invalid forward path: absolute URL would escape target host")
	}
	if strings.Contains(requestURL.Path, "//") {
		return fmt.Errorf("invalid forward path: contains protocol-relative segment")
	}
	return nil
}

func joinForwardPath(pathPrefix string, requestURL *url.URL) (string, error) {
	prefixSegments := pathSegments(pathPrefix)
	requestSegments := pathSegments("")
	if requestURL != nil {
		requestSegments = pathSegments(requestURL.Path)
	}
	return url.JoinPath("/", append(prefixSegments, requestSegments...)...)
}

func resolveForwardReference(base *url.URL, requestURL *url.URL, joinedPath string) (*url.URL, error) {
	ref := &url.URL{Path: joinedPath}
	if requestURL != nil && requestURL.RawQuery != "" {
		ref.RawQuery = requestURL.RawQuery
	}

	resolved := base.ResolveReference(ref)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return nil, fmt.Errorf("invalid forward URL: resolved URL would escape target host")
	}
	return resolved, nil
}

func (cfg ForwardConfig) failInvalidForwardURL(err error) error {
	wrapped := fmt.Errorf("%w: %w", util.ErrUnrecoverable, err)
	cfg.logInvalidForwardURL(err.Error())
	cfg.reportError(wrapped, "build_forward_url")
	return wrapped
}

func pathSegments(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func StatusCodeError(statusCode int) error {
	if statusCode >= 500 {
		return fmt.Errorf("remote server returned status %d", statusCode)
	}
	if statusCode >= 400 {
		return fmt.Errorf("%w: remote server returned status %d", util.ErrUnrecoverable, statusCode)
	}
	return nil
}

func HTTPToServiceResponse(resp *http.Response, body []byte) *entity.HttpServiceResponse {
	return &entity.HttpServiceResponse{
		ResponseMeta: &entity.HttpResponseMeta{
			OriginalResponse: resp,
			Status:           resp.Status,
			StatusCode:       resp.StatusCode,
			Proto:            resp.Proto,
			ProtoMajor:       resp.ProtoMajor,
			ProtoMinor:       resp.ProtoMinor,
			TransferEncoding: resp.TransferEncoding,
			Header:           resp.Header,
			Trailer:          resp.Trailer,
		},
		Body: body,
	}
}

// ReadLimitedBody reads from r up to limit bytes; limit+1 byte is read to detect overflow.
func ReadLimitedBody(r io.Reader, limit int64, cfg ForwardConfig) ([]byte, error) {
	if limit <= 0 {
		limit = cfg.maxBodyBytes()
	}
	lr := io.LimitReader(r, limit+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", util.ErrUnrecoverable, err)
	}
	if int64(len(body)) > limit {
		return nil, cfg.failBodyLimitExceeded(limit)
	}
	return body, nil
}

func (cfg ForwardConfig) failBodyLimitExceeded(limit int64) error {
	overflow := fmt.Errorf("%w: upstream response body exceeds limit of %d bytes", util.ErrUnrecoverable, limit)
	cfg.logLimitExceeded(limit)
	cfg.reportError(overflow, "read_response_body")
	return overflow
}

func ReadAndValidateResponse(resp *http.Response, cfg ForwardConfig) (*entity.HttpServiceResponse, error) {
	defer resp.Body.Close()

	body, err := ReadLimitedBody(resp.Body, cfg.maxBodyBytes(), cfg)
	if err != nil {
		return nil, err
	}

	if err := StatusCodeError(resp.StatusCode); err != nil {
		return nil, err
	}

	return HTTPToServiceResponse(resp, body), nil
}
