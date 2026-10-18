package remoteforward

import (
	"strings"

	"github.com/QueerGlobal/hub-framework/util"
)

// ShouldReportExhaustedHTTPFailure reports whether a failed Apply after backoff
// should emit a Sentry http_request event. Client errors and errors already
// reported at an earlier stage are excluded.
func ShouldReportExhaustedHTTPFailure(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), util.UnrecoverableErrorMsg) {
		return false
	}
	return strings.Contains(err.Error(), "all retry attempts failed")
}
