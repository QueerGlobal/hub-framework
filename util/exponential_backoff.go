package util

import (
	"math"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var UnrecoverableErrorMsg = "error is unrecoverable - backoff cancelled"

// MaxBackoffDelay is the hard ceiling for scaled retry delays (overflow protection).
const MaxBackoffDelay = time.Duration(math.MaxInt64)

// DefaultUncappedMaxDelay is the default cap when BackoffConfig.MaxDelay is unset.
const DefaultUncappedMaxDelay = 5 * time.Minute

type BackoffConfig struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	MaxRetries   int
	Logger       *zerolog.Logger
	TargetURL    string
}

type Backoff struct {
	config BackoffConfig
	sleep  func(time.Duration)
}

func NewBackoff(config BackoffConfig) *Backoff {
	return &Backoff{
		config: config,
		sleep:  time.Sleep,
	}
}

func (b *Backoff) maxAttempts() int {
	if b.config.MaxRetries <= 0 {
		return 1
	}
	return b.config.MaxRetries
}

func jitterForDelay(delay time.Duration) time.Duration {
	jitterRange := int64(delay / 2)
	if jitterRange <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(jitterRange))
}

// capDelay limits delay to maxDelay when maxDelay is positive; otherwise DefaultUncappedMaxDelay applies.
func capDelay(delay, maxDelay time.Duration) time.Duration {
	delay = clampDelayMagnitude(delay)
	limit := effectiveMaxDelay(maxDelay)
	if delay > limit {
		return limit
	}
	return delay
}

func effectiveMaxDelay(maxDelay time.Duration) time.Duration {
	if maxDelay > 0 {
		return maxDelay
	}
	return DefaultUncappedMaxDelay
}

func clampDelayMagnitude(delay time.Duration) time.Duration {
	if delay < 0 {
		return 0
	}
	return delay
}

func addDelay(a, b time.Duration) time.Duration {
	a = clampDelayMagnitude(a)
	b = clampDelayMagnitude(b)
	if a > MaxBackoffDelay-b {
		return MaxBackoffDelay
	}
	return a + b
}

func multiplyBackoffDelay(delay time.Duration, multiplier float64, maxDelay time.Duration) time.Duration {
	delay = clampDelayMagnitude(delay)
	if multiplier <= 0 || delay == 0 {
		return capDelay(delay, maxDelay)
	}
	scaled := float64(int64(delay)) * multiplier
	if scaled < 0 || scaled >= float64(math.MaxInt64) {
		return capDelay(MaxBackoffDelay, maxDelay)
	}
	return capDelay(time.Duration(int64(scaled)), maxDelay)
}

var (
	defaultBackoffLoggerOnce sync.Once
	defaultBackoffLogger     zerolog.Logger
)

func defaultLogger() zerolog.Logger {
	defaultBackoffLoggerOnce.Do(func() {
		defaultBackoffLogger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	})
	return defaultBackoffLogger
}

func (b *Backoff) logger() *zerolog.Logger {
	if b.config.Logger != nil {
		return b.config.Logger
	}
	log := defaultLogger()
	return &log
}

// TargetHostTag returns the host from a URL for logging (host:port or [ipv6]:port; no userinfo, path, or query).
func TargetHostTag(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}

func (b *Backoff) logRetry(attempt int, err error, sleepDelay time.Duration) {
	event := b.logger().Warn().
		Err(err).
		Int("attempt", attempt).
		Dur("retry_in", sleepDelay)
	if host := TargetHostTag(b.config.TargetURL); host != "" {
		event = event.Str("target_host", host)
	}
	event.Msg("backoff retry scheduled")
}

func (b *Backoff) logUnrecoverable(err error) {
	event := b.logger().Error().Err(err)
	if host := TargetHostTag(b.config.TargetURL); host != "" {
		event = event.Str("target_host", host)
	}
	event.Msg("unrecoverable error, backoff cancelled")
}

func (b *Backoff) ExecuteWithBackoff(operation func() error) error {
	baseDelay := b.config.InitialDelay

	var err error
	maxAttempts := b.maxAttempts()
	for i := 0; i < maxAttempts; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), UnrecoverableErrorMsg) {
			b.logUnrecoverable(err)
			return err
		}

		if i == maxAttempts-1 {
			break
		}

		sleepDelay := addDelay(baseDelay, jitterForDelay(baseDelay))
		sleepDelay = capDelay(sleepDelay, b.config.MaxDelay)

		b.logRetry(i+1, err, sleepDelay)
		b.sleep(sleepDelay)

		baseDelay = multiplyBackoffDelay(baseDelay, b.config.Multiplier, b.config.MaxDelay)
	}

	return errors.Wrap(err, "all retry attempts failed")
}
