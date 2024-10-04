package api

import (
	"context"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

// We duplicate the Target interface from the entity package to
// allow for the evolution of the API without breaking changes.
type Target interface {
	Apply(ctx context.Context, request ServiceRequest) (ServiceResponse, error)
}

// TargetAdapter wraps an api.Target and implements entity.Target
type TargetAdapter struct {
	exportedTarget Target
}

// NewTargetAdapter creates a new TargetAdapter
func NewTargetAdapter(target Target) *TargetAdapter {
	return &TargetAdapter{exportedTarget: target}
}

// Ensure TargetAdapter implements entity.Target
var _ entity.Target = (*TargetAdapter)(nil)

// Apply implements entity.Target
func (a *TargetAdapter) Apply(ctx context.Context, request entity.ServiceRequest) (entity.ServiceResponse, error) {
	var rqst ServiceRequest = request.(ServiceRequest)

	return a.exportedTarget.Apply(ctx, rqst)
}

// ConvertToEntityTarget converts api.Target to entity.Target
func ConvertToEntityTarget(target Target) entity.Target {
	return NewTargetAdapter(target)
}
