package util

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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

func TestCapDelay(t *testing.T) {
	tests := []struct {
		name     string
		delay    time.Duration
		maxDelay time.Duration
		want     time.Duration
	}{
		{name: "no cap when maxDelay zero", delay: 500 * time.Millisecond, maxDelay: 0, want: 500 * time.Millisecond},
		{name: "no cap when maxDelay negative", delay: time.Second, maxDelay: -1, want: time.Second},
		{name: "clamp when over cap", delay: 200 * time.Millisecond, maxDelay: 100 * time.Millisecond, want: 100 * time.Millisecond},
		{name: "unchanged under cap", delay: 50 * time.Millisecond, maxDelay: 100 * time.Millisecond, want: 50 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, capDelay(tt.delay, tt.maxDelay))
		})
	}
}

func TestExecuteWithBackoff_MaxDelayTable(t *testing.T) {
	rand.Seed(99)

	tests := []struct {
		name       string
		maxDelay   time.Duration
		wantGrowth bool
	}{
		{name: "no cap waits grow", maxDelay: 0, wantGrowth: true},
		{name: "cap clamps waits", maxDelay: 60 * time.Millisecond, wantGrowth: false},
	}

	const initial = 50 * time.Millisecond
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBackoff(BackoffConfig{
				InitialDelay: initial,
				MaxDelay:     tt.maxDelay,
				Multiplier:   2,
				MaxRetries:   4,
			})
			var sleeps []time.Duration
			b.sleep = func(d time.Duration) {
				sleeps = append(sleeps, d)
			}

			err := b.ExecuteWithBackoff(func() error { return fmt.Errorf("fail") })
			require.Error(t, err)
			require.GreaterOrEqual(t, len(sleeps), 2)

			if tt.wantGrowth {
				require.Greater(t, sleeps[len(sleeps)-1], initial, "delays should grow without a cap")
			} else {
				for _, d := range sleeps {
					require.LessOrEqual(t, d, tt.maxDelay)
				}
			}
		})
	}
}

func TestExecuteWithBackoff_LogsTargetHostRedacted(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf).With().Timestamp().Logger()
	b := NewBackoff(BackoffConfig{
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
		Multiplier:   1,
		MaxRetries:   2,
		Logger:       &log,
		TargetURL:    "http://apiuser:topsecret@upstream.example:8080/v1?token=leaked",
	})
	b.sleep = func(time.Duration) {}

	err := b.ExecuteWithBackoff(func() error { return fmt.Errorf("fail") })
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "upstream.example")
	assert.Contains(t, out, "backoff retry scheduled")
	assert.NotContains(t, out, "topsecret")
	assert.NotContains(t, out, "apiuser")
	assert.NotContains(t, out, "token=leaked")
}

func TestTargetHostTag_StripsCredentialsAndQuery(t *testing.T) {
	assert.Equal(t, "upstream.example", TargetHostTag("http://user:pass@upstream.example/path?q=1"))
	assert.Equal(t, "upstream.example:9090", TargetHostTag("http://user:pass@upstream.example:9090/path?q=1"))
}

func TestExecuteWithBackoff_UncappedDelayDoesNotOverflow(t *testing.T) {
	rand.Seed(7)

	const initial = time.Second
	b := NewBackoff(BackoffConfig{
		InitialDelay: initial,
		MaxDelay:     0,
		Multiplier:   2,
		MaxRetries:   40,
	})

	var sleeps []time.Duration
	b.sleep = func(d time.Duration) {
		require.GreaterOrEqual(t, int64(d), int64(0), "sleep delay must not be negative")
		require.LessOrEqual(t, d, MaxBackoffDelay)
		sleeps = append(sleeps, d)
	}

	err := b.ExecuteWithBackoff(func() error { return fmt.Errorf("fail") })
	require.Error(t, err)
	require.Len(t, sleeps, 39)

	for _, d := range sleeps {
		require.GreaterOrEqual(t, int64(d), int64(0))
	}
	require.Equal(t, MaxBackoffDelay, sleeps[len(sleeps)-1])
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
