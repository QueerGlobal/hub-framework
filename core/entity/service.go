package entity

import (
	"context"
	"fmt"
	"time"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
)

// ErrMethodNotConfigured is returned when a requested HTTP method is not configured for a service.
var ErrMethodNotConfigured error = fmt.Errorf("error not configured")

// Service represents a single service or DDD-style aggregate. It consists of handlers
// with incoming and outgoing workflows, and a target for each allowed HTTP method.
type Service struct {
	Name           string                  // Name of the service
	SchemaName     string                  // Name of the schema used by the service
	SchemaVersion  string                  // Version of the schema
	APIName        string                  // Name of the API this service belongs to
	IsPublic       bool                    // Indicates if the service is publicly accessible
	ServiceTimeout *time.Duration          // Timeout for service operations
	Methods        map[HTTPMethod]*Handler // Map of HTTP methods to their respective handlers
}

// Handler defines the structure for handling a specific HTTP method within a service.
type Handler struct {
	InboundWorkflow  Workflow // Workflow to be applied to incoming requests
	OutboundWorkflow Workflow // Workflow to be applied to outgoing responses
	Target           Target   // The target operation to be executed
}

// NewService creates and returns a new Service instance.
//
// Parameters:
//   - apiName: Name of the API this service belongs to.
//   - name: Name of the service.
//   - schemaName: Name of the schema used by the service.
//   - schemaVersion: Version of the schema.
//   - public: Indicates if the service is publicly accessible.
//
// Returns:
//   - *Service: A pointer to the newly created Service.
//   - error: Always nil in the current implementation.
func NewService(apiName, name, schemaName, schemaVersion string, public bool) (*Service, error) {
	service := Service{
		Name:          name,
		SchemaName:    schemaName,
		SchemaVersion: schemaVersion,
		APIName:       apiName,
		IsPublic:      public,
		Methods:       make(map[HTTPMethod]*Handler),
	}
	return &service, nil
}

// DoRequest processes an incoming service request by applying the appropriate workflows and target operation.
//
// Parameters:
//   - ctx: The context for the request.
//   - request: A pointer to the ServiceRequest to be processed.
//
// Returns:
//   - error: Any error encountered during processing, or nil if successful.
func (service *Service) DoRequest(ctx context.Context, request *ServiceRequest) error {
	if request == nil {
		return domainerr.ErrEmptyInput
	}

	var cancel context.CancelFunc
	if service.ServiceTimeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *service.ServiceTimeout)
		defer cancel()
	}

	method := request.Method

	handler, ok := service.Methods[method]
	if !ok {
		return fmt.Errorf("method %s not found for service %s, %w", method, request.ServiceName, ErrMethodNotConfigured)
	}

	if handler.InboundWorkflow != nil {
		err := handler.InboundWorkflow.Apply(ctx, request)
		if err != nil {
			return err
		}
	}

	var response *ServiceResponse
	var err error

	if handler.Target == nil {
		return domainerr.ErrTargetNotConfigured
	}

	response, err = handler.Target.Apply(ctx, request)
	if err != nil {
		return err
	}

	if response == nil {
		return domainerr.ErrEmptyResponse
	}

	request.Response = response

	if handler.OutboundWorkflow != nil {
		err := handler.OutboundWorkflow.Apply(ctx, request)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetHandler assigns a Handler to a specific HTTP method for the service.
//
// Parameters:
//   - method: The HTTP method to assign the handler to.
//   - handler: A pointer to the Handler to be assigned.
func (service *Service) SetHandler(method HTTPMethod, handler *Handler) {
	service.Methods[method] = handler
}

// GetHandlers returns a map of all configured HTTP methods and their handlers for the service.
//
// Returns:
//   - map[HTTPMethod]*Handler: A map of HTTP methods to their respective handlers.
func (service *Service) GetHandlers() map[HTTPMethod]*Handler {
	return service.Methods
}
