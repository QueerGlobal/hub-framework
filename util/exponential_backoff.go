package util

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var UnrecoverableErrorMsg = "error is unrecoverable - backoff cancelled"

type BackoffConfig struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	MaxRetries   int
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

func capDelay(delay, maxDelay time.Duration) time.Duration {
	if delay > maxDelay {
		return maxDelay
	}
	return delay
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
			fmt.Println("Unrecoverable error encountered, canceling backoff:", err) // TODO - replace this with proper logging
			return err
		}

		if i == maxAttempts-1 {
			break
		}

		sleepDelay := baseDelay + jitterForDelay(baseDelay)
		sleepDelay = capDelay(sleepDelay, b.config.MaxDelay)

		fmt.Printf("Attempt %d failed: %v. Retrying in %v...\n", i+1, err, sleepDelay) // TODO - replace this with proper logging
		b.sleep(sleepDelay)

		baseDelay = time.Duration(float64(baseDelay) * b.config.Multiplier)
		baseDelay = capDelay(baseDelay, b.config.MaxDelay)
	}

	return errors.Wrap(err, "all retry attempts failed")
}
