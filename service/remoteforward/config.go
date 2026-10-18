package remoteforward

import (
	"fmt"
	"net/url"

	"github.com/rs/zerolog"
)

// ForwardSettings is the input parameter object for constructing ForwardConfig.
type ForwardSettings struct {
	MaxResponseBodyBytes int64
	Logger               *zerolog.Logger
	Sentry               SentryConfig
	UpstreamHost         string
}

// ForwardConfig holds runtime forwarding options (logging, limits, error reporting).
type ForwardConfig struct {
	maxResponseBodyBytes int64
	logger               zerolog.Logger
	reporter             ErrorReporter
	targetHostTag        string
	shutdown             func()
}

// invalidForwardHostError returns a safe error when host cannot be parsed (never includes credentials or wrapped parse text).
func invalidForwardHostError(host string) error {
	return fmt.Errorf("invalid forward host %q", redactedHostURL(host))
}

// NewForwardConfig builds a ForwardConfig with default JSON stderr logging when Logger is nil.
func NewForwardConfig(settings ForwardSettings) (ForwardConfig, error) {
	logger := settings.Logger
	if logger == nil {
		defaultLog := DefaultLogger()
		logger = &defaultLog
	}

	reporter, shutdown, err := NewSentryReporter(settings.Sentry)
	if err != nil {
		return ForwardConfig{}, err
	}

	return ForwardConfig{
		maxResponseBodyBytes: settings.MaxResponseBodyBytes,
		logger:               *logger,
		reporter:             reporter,
		targetHostTag:        TargetHostTag(settings.UpstreamHost),
		shutdown:             shutdown,
	}, nil
}

// NewForwardConfigResilient returns ForwardConfig and logs a warning if Sentry initialization fails.
func NewForwardConfigResilient(settings ForwardSettings) ForwardConfig {
	cfg, err := NewForwardConfig(settings)
	if err == nil {
		return cfg
	}
	logger := settings.Logger
	if logger == nil {
		defaultLog := DefaultLogger()
		logger = &defaultLog
	}
	logger.Warn().Err(err).Msg("Sentry init failed for remote forwarder; continuing without error reporting")
	settings.Sentry = SentryConfig{}
	cfg, err = NewForwardConfig(settings)
	if err != nil {
		panic(err)
	}
	return cfg
}

// Shutdown flushes pending Sentry events when a DSN-backed reporter is configured.
func (c ForwardConfig) Shutdown() {
	if c.shutdown != nil {
		c.shutdown()
	}
}

func (c ForwardConfig) maxBodyBytes() int64 {
	if c.maxResponseBodyBytes > 0 {
		return c.maxResponseBodyBytes
	}
	return DefaultMaxResponseBodyBytes
}

func (c ForwardConfig) reportError(err error, stage string) {
	if c.reporter == nil {
		return
	}
	tags := map[string]string{"stage": stage}
	if c.targetHostTag != "" {
		tags["target_host"] = c.targetHostTag
	}
	c.reporter.ReportError(err, tags)
}

// ReportHTTPFailure sends a redacted forward HTTP failure to the configured error reporter.
func (c ForwardConfig) ReportHTTPFailure(err error) {
	c.reportError(err, "http_request")
}

func (c ForwardConfig) logLimitExceeded(limit int64) {
	c.logger.Warn().Int64("limitBytes", limit).Str("target_host", c.targetHostTag).Msg("upstream response body exceeded configured limit")
}

func (c ForwardConfig) logInvalidForwardURL(reason string) {
	c.logger.Warn().Str("reason", reason).Str("target_host", c.targetHostTag).Msg("invalid remote forward URL")
}

// TargetHostTag returns the upstream hostname for monitoring tags (no credentials, path, or query).
func TargetHostTag(hostURL string) string {
	parsed, err := url.Parse(hostURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Hostname()
}

// redactedHostURL returns scheme and hostname for error messages (no userinfo, path, or query).
func redactedHostURL(hostURL string) string {
	parsed, err := url.Parse(hostURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "<invalid-host>"
	}
	return parsed.Scheme + "://" + parsed.Hostname()
}
