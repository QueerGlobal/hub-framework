package api

import (
	"github.com/QueerGlobal/hub-framework/core/entity"
)

type TaskConstructor interface {
	New(config map[string]any) (Task, error)
}

// TaskConstructorFunc is a function type that implements TaskConstructor
type TaskConstructorFunc func(config map[string]any) (Task, error)

// New implements the TaskConstructor interface for TaskConstructorFunc
func (f TaskConstructorFunc) New(config map[string]any) (Task, error) {
	return f(config)
}

func RegisterTaskType(name string, taskConstructor TaskConstructor) {
	var entityTaskConstructor entity.TaskConstructorFunc
	// Create an adapter function to convert TaskConstructor to entity.TaskConstructor
	entityTaskConstructor = func(config map[string]any) (entity.Task, error) {
		task, err := taskConstructor.New(config)
		if err != nil {
			return nil, err
		}

		taskAdapter := NewTaskAdapter(task)

		return taskAdapter, nil
	}

	entity.RegisterTaskType(name, entityTaskConstructor)
}

func GetTask(taskName string, config map[string]interface{}) (entity.Task, error) {

	return entity.GetTask(taskName, config)
}

// TaskConstructorFromFunction creates a TaskConstructorFunc from a function
// with the signature func(map[string]interface{}) (T, error), where T implements Task
func TaskConstructorFromFunction(
	fn func(map[string]interface{}) (Task, error),
) TaskConstructorFunc {
	return TaskConstructorFunc(fn)
}
