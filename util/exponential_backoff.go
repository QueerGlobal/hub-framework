package util

import (
	"fmt"
	"github.com/pkg/errors"
	"math/rand"
	"strings"
	"time"
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
}

func NewBackoff(config BackoffConfig) *Backoff {
	return &Backoff{config: config}
}

func (b *Backoff) ExecuteWithBackoff(operation func() error) error {
	delay := b.config.InitialDelay

	var err error
	for i := 0; i < b.config.MaxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), UnrecoverableErrorMsg) {
			fmt.Println("Unrecoverable error encountered, canceling backoff:", err) // TODO - replace this with proper logging
			return err
		}

		fmt.Printf("Attempt %d failed: %v. Retrying in %v...\n", i+1, err, delay) // TODO - replace this with proper logging
		time.Sleep(delay)

		// Calculate next delay with jitter
		delay = time.Duration(float64(delay) * b.config.Multiplier)
		if delay > b.config.MaxDelay {
			delay = b.config.MaxDelay
		}
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		delay += jitter
	}

	return errors.Wrap(err, "all retry attempts failed")
}
