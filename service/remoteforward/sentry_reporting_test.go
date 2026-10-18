package remoteforward

import (
	"fmt"
	"testing"

	"github.com/QueerGlobal/hub-framework/util"
	"github.com/stretchr/testify/assert"
)

func TestShouldReportExhaustedHTTPFailure(t *testing.T) {
	assert.False(t, ShouldReportExhaustedHTTPFailure(nil))
	assert.False(t, ShouldReportExhaustedHTTPFailure(fmt.Errorf("%s: status 404", util.UnrecoverableErrorMsg)))
	assert.False(t, ShouldReportExhaustedHTTPFailure(fmt.Errorf("%s: invalid host", util.UnrecoverableErrorMsg)))
	assert.True(t, ShouldReportExhaustedHTTPFailure(fmt.Errorf("all retry attempts failed: remote server returned status 503")))
}
