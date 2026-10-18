package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackoffHTTPRetryEndToEnd(t *testing.T) {
	t.Run("retries then succeeds", func(t *testing.T) {
		var calls atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) < 3 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		t.Cleanup(ts.Close)

		client := &http.Client{Timeout: time.Second}
		b := util.NewBackoff(util.BackoffConfig{
			InitialDelay: time.Millisecond,
			MaxDelay:     5 * time.Millisecond,
			Multiplier:   2,
			MaxRetries:   5,
		})
		var body string
		err := b.ExecuteWithBackoff(func() error {
			resp, err := client.Get(ts.URL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 500 {
				return fmt.Errorf("upstream status %d", resp.StatusCode)
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%s: status %d", util.UnrecoverableErrorMsg, resp.StatusCode)
			}
			body = "ok"
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, int32(3), calls.Load())
		assert.Equal(t, "ok", body)
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		var calls atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		t.Cleanup(ts.Close)

		client := &http.Client{Timeout: time.Second}
		b := util.NewBackoff(util.BackoffConfig{
			InitialDelay: time.Millisecond,
			MaxDelay:     2 * time.Millisecond,
			Multiplier:   1,
			MaxRetries:   3,
		})
		err := b.ExecuteWithBackoff(func() error {
			resp, err := client.Get(ts.URL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 500 {
				return fmt.Errorf("upstream status %d", resp.StatusCode)
			}
			return nil
		})
		require.Error(t, err)
		assert.Equal(t, int32(3), calls.Load())
		assert.Contains(t, err.Error(), "all retry attempts failed")
	})
}
