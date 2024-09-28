package entity

import (
	"fmt"
	"sync"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
)

// TaskConstructor takes a config object and returns a configured
// Target instance.
type TaskConstructor interface {
	New(config map[string]any) (Task, error)
}

type TaskConstructorFunc func(config map[string]any) (Task, error)

func (f TaskConstructorFunc) New(config map[string]any) (Task, error) {
	return f(config)
}

// RegisteredWorkflowSteps allows us to register workflow steps at start-up
// in order to use them at start-up
type RegisteredTaskTypes map[string]TaskConstructor

var (
	onceRegisteredTasks sync.Once
	registeredTaskTypes RegisteredTaskTypes
)

// RegisteredRequestWorkflowSteps exposes a singleton object containing
// all registered worklfow steps which can be applied to service requests
func TaskRegistry() RegisteredTaskTypes {

	onceRegisteredTasks.Do(func() {

		registeredTaskTypes = make(RegisteredTaskTypes)

	})

	return registeredTaskTypes
}

func RegisterTaskType(name string,
	taskConstructor TaskConstructor) {

	TaskRegistry()[name] = taskConstructor
}

// GetTask retrieves a Task Constructor instance by its name,
// passes in a config and returns a configured Task instance.
// It returns an error if the task type is not registered.
//
// Parameters:
// - taskName: the name of the task to retrieve
// - config: a map containing the configuration for the task
//
// Returns:
// - Task: the configured Task instance
// - error: an error if the task is not registered
func GetTask(taskName string, config map[string]interface{}) (Task, error) {
	taskConstructor, ok := TaskRegistry()[taskName]
	if !ok {
		return nil, fmt.Errorf("unknown task %s: %w", taskName,
			domainerr.ErrWorkflowTaskNotRegistered)
	}

	task, err := taskConstructor.New(config)
	if err != nil {
		return nil, err
	}

	return task, nil
}
