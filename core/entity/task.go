package entity

import "context"

// Task is the interface for a task that can be performed as part of a workflow.
// Tasks applied to incoming requests and outgoing responses both accept and return
// ServiceRequest, but outgoing responses generally will work with the response
// object.
type Task interface {
	Name() string
	Apply(ctx context.Context, request *ServiceRequest) error
}
