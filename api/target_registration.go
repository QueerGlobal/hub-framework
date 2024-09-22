package api

import (
	"github.com/QueerGlobal/qg-hub/core/entity"
)

// TargetConstructor takes a config object and returns a configured
// Target instance.
type TargetConstructor interface {
	New(config map[string]any) entity.Target
}

// RegisteredWorkflowSteps allows us to register workflow steps at start-up
// in order to use them at start-up
type RegisteredTargets map[string]TargetConstructor

func RegisterTargetType(name string,
	targetConstructor TargetConstructor) {

	entity.RegisterTargetType(name, targetConstructor)
}

func GetTarget(targetName string, config map[string]interface{}) (entity.Target, error) {
	return entity.GetTarget(targetName, config)
}
