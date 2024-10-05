package entity

import (
	"context"
	"sort"

	"golang.org/x/exp/maps"
)

// Workflow is the interface for a workflow that can be applied to a ServiceRequest.
type Workflow interface {
	Apply(ctx context.Context, in ServiceRequest) error
}

// Workflow is a chain of workflow steps that can be applied sequentially
// according to priority.
type WorkflowTasks struct {
	Steps map[int][]*WorkflowStep
}

// NewWorkflow creates a new workflow
func NewWorkflowTasks(steps ...*WorkflowStep) *WorkflowTasks {

	tasks := WorkflowTasks{
		Steps: make(map[int][]*WorkflowStep),
	}
	for _, step := range steps {

		tasks.add(step)
	}
	return &tasks
}

// Add adds a workflow step to this workflow
func (w *WorkflowTasks) add(
	step *WorkflowStep) {

	precedence := step.Precedence

	transformations, ok := w.Steps[precedence]
	if !ok {
		transformations = []*WorkflowStep{}
	}
	transformations = append(transformations, step)
	w.Steps[precedence] = transformations

}

// Apply applies all transformations in this chain, in order of precedence
func (chain *WorkflowTasks) Apply(
	ctx context.Context,
	in ServiceRequest) error {

	keys := maps.Keys(chain.Steps)
	sort.Ints(keys)

	rqst := in
	for _, key := range keys {

		steps := chain.Steps[key]

		for _, step := range steps {
			if err := step.GetTask().Apply(ctx, rqst); err != nil {
				return err
			}
		}
	}
	return nil
}
