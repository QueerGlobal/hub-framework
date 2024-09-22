package requesthandler

import (
	"sync"
)

type DefaultValues struct {
	ServiceName        string
	Success            string
	FailureUnspecified string
}

var defaults *DefaultValues
var onceSetDefaults sync.Once

func GetDefaults() *DefaultValues {

	onceSetDefaults.Do(func() {
		setDefaults()
	})
	return defaults
}

func setDefaults() {

	vals := DefaultValues{}
	vals.ServiceName = "Hub"
	vals.Success = "Success"
	vals.FailureUnspecified = "500"
	defaults = &vals

}
