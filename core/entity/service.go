package entity

import (
	"context"
	"fmt"
	"time"

	domainerr "github.com/QueerGlobal/qg-hub/core/entity/error"
)

var ErrMethodNotConfigured error = fmt.Errorf("error not configured")

type Service struct {
	Name           string
	SchemaName     string
	SchemaVersion  string
	APIName        string
	IsPublic       bool
	ServiceTimeout *time.Duration
	Methods        map[HTTPMethod]*Handler
}

type Handler struct {
	InboundWorkflow  Workflow
	OutboundWorkflow Workflow
	Target           Target
}

func NewService(apiName, name, schemaName, schemaVersion string,
	public bool) (*Service, error) {
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

// doRequest first applies a series of workflow steps to an incoming
// request, then
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

func (service *Service) SetHandler(method HTTPMethod, handler *Handler) {
	service.Methods[method] = handler
}

func (service *Service) GetHandlers() map[HTTPMethod]*Handler {
	return service.Methods
}
