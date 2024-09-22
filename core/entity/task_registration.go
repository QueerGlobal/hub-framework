package entity

import (
	"fmt"
	"sync"

	domainerr "github.com/QueerGlobal/qg-hub/core/entity/error"
)

// TaskConstructor takes a config object and returns a configured
// Target instance.
type TaskConstructor interface {
	New(config map[string]any) Task
}

type TaskConstructorFunc func(config map[string]any) Task

func (f TaskConstructorFunc) New(config map[string]any) Task {
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

func GetTask(taskName string, config map[string]interface{}) (Task, error) {
	taskConstructor, ok := TaskRegistry()[taskName]
	if !ok {
		return nil, fmt.Errorf("unknown task %s: %w", taskName,
			domainerr.ErrWorkflowTaskNotRegistered)
	}

	task := taskConstructor.New(config)

	return task, nil
}
