package api

import (
	"github.com/QueerGlobal/hub-framework/core/entity"
)

// TargetConstructor takes a config object and returns a configured
// Target instance.
type TargetConstructor interface {
	New(config map[string]any) (Target, error)
}

// TaskConstructorFunc is a function type that implements TaskConstructor
type TargetConstructorFunc func(config map[string]any) (Target, error)

// New implements the TaskConstructor interface for TaskConstructorFunc
func (f TargetConstructorFunc) New(config map[string]any) (Target, error) {
	return f(config)
}

// RegisteredTargets allows us to register targets at start-up
type RegisteredTargets map[string]TargetConstructor

func RegisterTargetType(name string, targetConstructor TargetConstructor) {
	var entityTargetConstructor entity.TargetConstructorFunc
	entityTargetConstructor = func(config map[string]any) (entity.Target, error) {
		target, err := targetConstructor.New(config)
		if err != nil {
			return nil, err
		}

		tgt := NewTargetAdapter(target)
		return tgt, nil
	}

	entity.RegisterTargetType(name, entityTargetConstructor)
}

func GetTarget(targetName string, config map[string]interface{}) (entity.Target, error) {
	return entity.GetTarget(targetName, config)
}
