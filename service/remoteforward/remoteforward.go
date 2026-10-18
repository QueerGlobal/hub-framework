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
	"github.com/rs/zerolog"
)

const (
	DefaultClientTimeout         = 30 * time.Second
	DefaultMaxResponseBodyBytes  = 10 << 20 // 10 MiB
)

// ErrorReporter captures unexpected forward failures (for example Sentry when configured).
type ErrorReporter interface {
	ReportError(err error, tags map[string]string)
}

type noopErrorReporter struct{}

func (noopErrorReporter) ReportError(error, map[string]string) {}

// SentryConfig selects whether forward errors are reported upstream.
type SentryConfig struct {
	DSN string
}

// NewErrorReporter returns a no-op reporter when DSN is empty; otherwise a DSN-backed reporter hook.
func NewErrorReporter(cfg SentryConfig) ErrorReporter {
	if strings.TrimSpace(cfg.DSN) == "" {
		return noopErrorReporter{}
	}
	return &dsnErrorReporter{dsn: cfg.DSN}
}

// dsnErrorReporter is the extension point for Sentry (or similar) when a DSN is configured.
// Reporting is intentionally lightweight so callers can swap in a real SDK at the application layer.
type dsnErrorReporter struct {
	dsn string
}

func (r *dsnErrorReporter) ReportError(err error, tags map[string]string) {
	// Hook point: integrate sentry-go or another backend using r.dsn when wiring the application.
	_ = r.dsn
	_ = err
	_ = tags
}

// Options configures shared remote forward behavior.
type Options struct {
	MaxResponseBodyBytes int64
	Logger               *zerolog.Logger
	ErrorReporter        ErrorReporter
}

func (o Options) maxBodyBytes() int64 {
	if o.MaxResponseBodyBytes > 0 {
		return o.MaxResponseBodyBytes
	}
	return DefaultMaxResponseBodyBytes
}

func (o Options) report(err error, tags map[string]string) {
	if o.ErrorReporter == nil {
		return
	}
	o.ErrorReporter.ReportError(err, tags)
}

func (o Options) logLimitExceeded(limit int64) {
	if o.Logger == nil {
		return
	}
	o.Logger.Warn().Int64("limitBytes", limit).Msg("upstream response body exceeded configured limit")
}

func (o Options) logInvalidForwardURL(reason string) {
	if o.Logger == nil {
		return
	}
	o.Logger.Warn().Str("reason", reason).Msg("invalid remote forward URL")
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}
	return &http.Client{Timeout: timeout}
}

// BuildForwardURL resolves host, path prefix, and request URL into a single forward target.
// The result always shares the host and scheme of host; request paths cannot escape to another host.
func BuildForwardURL(host, pathPrefix string, requestURL *url.URL, opts Options) (string, error) {
	base, err := url.Parse(host)
	if err != nil {
		wrapped := fmt.Errorf("invalid forward host %q: %w", host, err)
		opts.logInvalidForwardURL(wrapped.Error())
		opts.report(wrapped, map[string]string{"stage": "build_forward_url"})
		return "", wrapped
	}
	if base.Scheme == "" || base.Host == "" {
		wrapped := fmt.Errorf("invalid forward host %q: scheme and host are required", host)
		opts.logInvalidForwardURL(wrapped.Error())
		opts.report(wrapped, map[string]string{"stage": "build_forward_url"})
		return "", wrapped
	}

	if requestURL != nil {
		if requestURL.IsAbs() && requestURL.Host != "" && requestURL.Host != base.Host {
			wrapped := fmt.Errorf("invalid forward path: absolute URL would escape target host")
			opts.logInvalidForwardURL(wrapped.Error())
			opts.report(wrapped, map[string]string{"stage": "build_forward_url"})
			return "", wrapped
		}
		if strings.Contains(requestURL.Path, "//") {
			wrapped := fmt.Errorf("invalid forward path: contains protocol-relative segment")
			opts.logInvalidForwardURL(wrapped.Error())
			opts.report(wrapped, map[string]string{"stage": "build_forward_url"})
			return "", wrapped
		}
	}

	prefixSegments := pathSegments(pathPrefix)
	requestSegments := pathSegments("")
	if requestURL != nil {
		requestSegments = pathSegments(requestURL.Path)
	}

	joinedPath, err := url.JoinPath("/", append(prefixSegments, requestSegments...)...)
	if err != nil {
		wrapped := fmt.Errorf("invalid forward path: %w", err)
		opts.logInvalidForwardURL(wrapped.Error())
		opts.report(wrapped, map[string]string{"stage": "build_forward_url"})
		return "", wrapped
	}

	ref := &url.URL{Path: joinedPath}
	if requestURL != nil && requestURL.RawQuery != "" {
		ref.RawQuery = requestURL.RawQuery
	}

	resolved := base.ResolveReference(ref)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		wrapped := fmt.Errorf("invalid forward URL: resolved URL would escape target host")
		opts.logInvalidForwardURL(wrapped.Error())
		opts.report(wrapped, map[string]string{"stage": "build_forward_url"})
		return "", wrapped
	}

	return resolved.String(), nil
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
		return fmt.Errorf("%s: remote server returned status %d", util.UnrecoverableErrorMsg, statusCode)
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
func ReadLimitedBody(r io.Reader, limit int64, opts Options) ([]byte, error) {
	if limit <= 0 {
		limit = opts.maxBodyBytes()
	}
	lr := io.LimitReader(r, limit+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", util.UnrecoverableErrorMsg, err)
	}
	if int64(len(body)) > limit {
		overflow := fmt.Errorf("%s: upstream response body exceeds limit of %d bytes", util.UnrecoverableErrorMsg, limit)
		opts.logLimitExceeded(limit)
		opts.report(overflow, map[string]string{"stage": "read_response_body"})
		return nil, overflow
	}
	return body, nil
}

func ReadAndValidateResponse(resp *http.Response, opts Options) (*entity.HttpServiceResponse, error) {
	defer resp.Body.Close()

	body, err := ReadLimitedBody(resp.Body, opts.maxBodyBytes(), opts)
	if err != nil {
		return nil, err
	}

	if err := StatusCodeError(resp.StatusCode); err != nil {
		return nil, err
	}

	return HTTPToServiceResponse(resp, body), nil
}
