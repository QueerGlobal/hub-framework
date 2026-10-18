package remoteforward

import (
	"fmt"
	"testing"

	"github.com/QueerGlobal/hub-framework/util"
	"github.com/stretchr/testify/assert"
)

func TestShouldReportExhaustedHTTPFailure(t *testing.T) {
	assert.False(t, ShouldReportExhaustedHTTPFailure(nil))
	assert.False(t, ShouldReportExhaustedHTTPFailure(fmt.Errorf("%w: status 404", util.ErrUnrecoverable)))
	assert.False(t, ShouldReportExhaustedHTTPFailure(fmt.Errorf("%w: invalid host", util.ErrUnrecoverable)))
	assert.True(t, ShouldReportExhaustedHTTPFailure(fmt.Errorf("%w: remote server returned status 503", util.ErrRetryAttemptsExhausted)))
}
