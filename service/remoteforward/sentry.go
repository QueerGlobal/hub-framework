package remoteforward

import (
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

const sentryFlushTimeout = 2 * time.Second

// ErrorReporter captures unexpected forward failures (for example Sentry when configured).
type ErrorReporter interface {
	ReportError(err error, tags map[string]string)
}

type noopErrorReporter struct{}

func (noopErrorReporter) ReportError(error, map[string]string) {}

// SentryConfig configures optional Sentry error reporting for forward failures.
type SentryConfig struct {
	DSN         string
	Environment string
	Release     string
	SampleRate  float64
	// Transport overrides the Sentry client transport (for example in tests).
	Transport sentry.Transport
}

// NewSentryReporter builds a Sentry-backed reporter when DSN is set; otherwise a no-op.
func NewSentryReporter(cfg SentryConfig) (ErrorReporter, func(), error) {
	return newSentryReporter(cfg, nil)
}

func newSentryReporter(cfg SentryConfig, transport sentry.Transport) (ErrorReporter, func(), error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return noopErrorReporter{}, func() {}, nil
	}

	sampleRate := cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 1.0
	}

	opts := sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		SampleRate:       sampleRate,
		AttachStacktrace: false,
	}
	if transport != nil {
		opts.Transport = transport
	} else if cfg.Transport != nil {
		opts.Transport = cfg.Transport
	}

	client, err := sentry.NewClient(opts)
	if err != nil {
		return nil, nil, err
	}

	reporter := &sentryReporter{client: client}
	shutdown := func() {
		client.Flush(sentryFlushTimeout)
	}
	return reporter, shutdown, nil
}

type sentryReporter struct {
	client *sentry.Client
}

func (r *sentryReporter) ReportError(err error, tags map[string]string) {
	if r.client == nil || err == nil {
		return
	}
	scope := sentry.NewScope()
	scope.SetLevel(sentry.LevelError)
	for key, value := range tags {
		scope.SetTag(key, value)
	}
	r.client.CaptureException(err, &sentry.EventHint{}, scope)
}
