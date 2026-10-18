package remoteforward

import (
	"errors"

	"github.com/QueerGlobal/hub-framework/util"
)

// ShouldReportExhaustedHTTPFailure reports whether a failed Apply after backoff
// should emit a Sentry http_request event. Client errors and errors already
// reported at an earlier stage are excluded.
func ShouldReportExhaustedHTTPFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, util.ErrUnrecoverable) {
		return false
	}
	return errors.Is(err, util.ErrRetryAttemptsExhausted)
}
