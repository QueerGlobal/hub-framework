package entity

import (
	"fmt"
	"sync"

	domainerr "github.com/QueerGlobal/qg-hub/core/entity/error"
)

// TargetConstructor takes a config object and returns a configured
// Target instance.
type TargetConstructor interface {
	New(config map[string]any) Target
}

type TargetConstructorFunc func(config map[string]any) Target

func (f TargetConstructorFunc) New(config map[string]any) Target {
	return f(config)
}

// RegisteredWorkflowSteps allows us to register workflow steps at start-up
// in order to use them at start-up
type RegisteredTargets map[string]TargetConstructor

var (
	onceRegisteredTargets sync.Once
	registeredTargets     RegisteredTargets
)

// RegisteredRequestWorkflowSteps exposes a singleton object containing
// all registered worklfow steps which can be applied to service requests
func TargetRegistry() RegisteredTargets {

	onceRegisteredTargets.Do(func() {

		registeredTargets = make(RegisteredTargets)

	})

	return registeredTargets
}
func RegisterTargetType(name string,
	targetConstructor TargetConstructor) {
	TargetRegistry()[name] = targetConstructor
}
func GetTarget(targetName string, config map[string]interface{}) (Target, error) {

	targetConstructor, ok := TargetRegistry()[targetName]
	if !ok {
		return nil, fmt.Errorf("unknown target %s: %w", targetName,
			domainerr.ErrTargetNotRegistered)
	}

	target := targetConstructor.New(config)

	return target, nil
}
