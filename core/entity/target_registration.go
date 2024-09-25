package entity

import (
	"fmt"
	"sync"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
)

// TargetConstructor takes a config object and returns a configured
// Target instance.
type TargetConstructor interface {
	New(config map[string]any) (Target, error)
}

type TargetConstructorFunc func(config map[string]any) (Target, error)

func (f TargetConstructorFunc) New(config map[string]any) (Target, error) {
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

// GetTarget retrieves a Target Constructor instance by its name,
// passes in a config and returns a configured Target instance.
// It returns an error if the target type is not registered.
//
// Parameters:
// - taskName: the name of the task to retrieve
// - config: a map containing the configuration for the task
//
// Returns:
// - Task: the configured Task instance
// - error: an error if the task is not registered
func GetTarget(targetName string, config map[string]interface{}) (Target, error) {

	targetConstructor, ok := TargetRegistry()[targetName]
	if !ok {
		return nil, fmt.Errorf("unknown target %s: %w", targetName,
			domainerr.ErrTargetNotRegistered)
	}

	target, err := targetConstructor.New(config)
	if err != nil {
		return nil, err
	}

	return target, nil
}
