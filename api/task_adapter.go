package api

import (
	"context"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

// We duplicate the Task interface from the entity package to
// allow for the evolution of the API without breaking changes.
type Task interface {
	Name() string
	Apply(ctx context.Context, request ServiceRequest) error
}

// TaskAdapter wraps an api.Task and implements entity.Task
type TaskAdapter struct {
	exportedTask Task
}

// NewTaskAdapter creates a new TaskAdapter
func NewTaskAdapter(task Task) *TaskAdapter {
	return &TaskAdapter{exportedTask: task}
}

// Ensure TaskAdapter implements entity.Task
var _ entity.Task = (*TaskAdapter)(nil)

// Name implements entity.Task
func (a *TaskAdapter) Name() string {
	return a.exportedTask.Name()
}

// Apply implements entity.Task
func (a *TaskAdapter) Apply(ctx context.Context, request entity.ServiceRequest) error {
	var rqst ServiceRequest = request.(ServiceRequest)

	return a.exportedTask.Apply(ctx, rqst)
}

// ConvertToEntityTask converts api.Task to entity.Task
func ConvertToEntityTask(task Task) entity.Task {
	return NewTaskAdapter(task)
}
