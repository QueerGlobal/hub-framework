package entity

import "context"

// HasPrecedence provides an interface for types(such as WorkflowSteps)
// that can be applied in order given a precedence value, where the lowest
// precedence value is executed first.
type HasPrecedence interface {
	Precedence() int // a value indicating precedence when a chain of
	// workflow steps are called in sequence. A lower value will cause
	// this workflow step to run earlier than a step with a
	// higher value.
}

// WorkflowStep provides an struct for transformations / work to be applied
// upon an incoming object of type T, with a precedence. This is intended
// top allow Tasks to be chained into workflows that can be applied
// upon incoming objects in a pre-specified order.
type WorkflowStep struct {
	Task          Task // the task to be applied
	Name          string
	Description   string
	TaskType      string
	Ref           string
	ExecutionType string
	Config        map[string]interface{}
	Precedence    int // a value indicating precedence within a chain of
	// workflow steps
}

func (w WorkflowStep) GetTask() Task {
	return w.Task
}

func (w WorkflowStep) Apply(ctx context.Context, request *ServiceRequest) error {
	return w.Task.Apply(ctx, request)
}

func (w WorkflowStep) GetPrecedence() int {
	return w.Precedence
}

func (w WorkflowStep) GetTaskName() string {
	return w.Task.Name()
}

func (w WorkflowStep) GetStepName() string {
	return w.Name
}
