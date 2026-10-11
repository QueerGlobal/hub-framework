package util

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestExecuteWithBackoff_ZeroConfigRunsOnce(t *testing.T) {
	b := NewBackoff(BackoffConfig{})

	var calls atomic.Int32
	err := b.ExecuteWithBackoff(func() error {
		calls.Add(1)
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
}

func TestExecuteWithBackoff_ZeroMaxRetriesStillAttemptsOnce(t *testing.T) {
	b := NewBackoff(BackoffConfig{MaxRetries: 0})
	b.sleep = func(time.Duration) {
		t.Fatal("should not sleep for a single attempt")
	}

	var calls atomic.Int32
	err := b.ExecuteWithBackoff(func() error {
		calls.Add(1)
		return fmt.Errorf("fail")
	})

	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load())
	require.Contains(t, err.Error(), "all retry attempts failed")
}

func TestExecuteWithBackoff_SuccessOnAttemptNStopsRetrying(t *testing.T) {
	b := NewBackoff(BackoffConfig{
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
		Multiplier:   1,
		MaxRetries:   5,
	})
	b.sleep = func(time.Duration) {}

	var calls atomic.Int32
	err := b.ExecuteWithBackoff(func() error {
		if calls.Add(1) < 3 {
			return fmt.Errorf("fail")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
}

func TestExecuteWithBackoff_UnrecoverableErrorStopsImmediately(t *testing.T) {
	b := NewBackoff(BackoffConfig{
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Second,
		Multiplier:   2,
		MaxRetries:   10,
	})
	b.sleep = func(time.Duration) {
		t.Fatal("should not sleep after unrecoverable error")
	}

	var calls atomic.Int32
	err := b.ExecuteWithBackoff(func() error {
		calls.Add(1)
		return errors.Errorf("%s: boom", UnrecoverableErrorMsg)
	})

	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load())
	require.Contains(t, err.Error(), UnrecoverableErrorMsg)
}

func TestExecuteWithBackoff_NoPanicWithZeroOrTinyDelays(t *testing.T) {
	rand.Seed(1)

	configs := []BackoffConfig{
		{InitialDelay: 0, MaxDelay: time.Second, Multiplier: 1, MaxRetries: 3},
		{InitialDelay: time.Nanosecond, MaxDelay: time.Nanosecond, Multiplier: 0, MaxRetries: 3},
		{InitialDelay: time.Millisecond, MaxDelay: 0, Multiplier: 2, MaxRetries: 3},
	}

	for _, cfg := range configs {
		cfg := cfg
		t.Run(fmt.Sprintf("%+v", cfg), func(t *testing.T) {
			b := NewBackoff(cfg)
			b.sleep = func(time.Duration) {}

			var calls atomic.Int32
			err := b.ExecuteWithBackoff(func() error {
				calls.Add(1)
				return fmt.Errorf("fail")
			})

			require.Error(t, err)
			maxAttempts := cfg.MaxRetries
			if maxAttempts <= 0 {
				maxAttempts = 1
			}
			require.Equal(t, int32(maxAttempts), calls.Load())
		})
	}
}

func TestExecuteWithBackoff_SleepsNeverExceedMaxDelay(t *testing.T) {
	rand.Seed(42)

	const maxDelay = 100 * time.Millisecond
	b := NewBackoff(BackoffConfig{
		InitialDelay: maxDelay,
		MaxDelay:     maxDelay,
		Multiplier:   3,
		MaxRetries:   20,
	})

	var sleeps []time.Duration
	b.sleep = func(d time.Duration) {
		sleeps = append(sleeps, d)
	}

	err := b.ExecuteWithBackoff(func() error {
		return fmt.Errorf("fail")
	})
	require.Error(t, err)
	require.NotEmpty(t, sleeps)

	for _, d := range sleeps {
		require.LessOrEqual(t, d, maxDelay, "sleep delay must not exceed MaxDelay")
	}
}

func TestExecuteWithBackoff_NoSleepAfterFinalAttempt(t *testing.T) {
	b := NewBackoff(BackoffConfig{
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
		Multiplier:   1,
		MaxRetries:   4,
	})

	var sleepCount atomic.Int32
	b.sleep = func(time.Duration) {
		sleepCount.Add(1)
	}

	err := b.ExecuteWithBackoff(func() error {
		return fmt.Errorf("fail")
	})
	require.Error(t, err)
	require.Equal(t, int32(3), sleepCount.Load(), "should sleep once less than total attempts")
}
