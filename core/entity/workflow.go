package entity

import (
	"context"
	"fmt"
	"sort"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

	tracer := otel.Tracer("workflow")
	ctx, span := tracer.Start(ctx, "WorkflowTasks.Apply")
	defer span.End()

	keys := maps.Keys(chain.Steps)
	sort.Ints(keys)

	rqst := in
	for _, key := range keys {
		steps := chain.Steps[key]

		for _, step := range steps {
			stepCtx, stepSpan := tracer.Start(ctx, "workflow.step",
				trace.WithAttributes(
					attribute.Int("precedence", key),
					attribute.String("step", step.Name),
				))

			err := step.GetTask().Apply(stepCtx, rqst)
			if err != nil {
				stepSpan.RecordError(err)
				stepSpan.SetStatus(codes.Error, err.Error())
				stepSpan.End()
				return err
			}

			stepSpan.SetStatus(codes.Ok, fmt.Sprintf("Step '%s' completed successfully", step.Name))
			stepSpan.End()
		}
	}
	return nil
}
