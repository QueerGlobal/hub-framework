package entity

import (
	"context"
	"net/http"
)

/*
Target represents the end destination of an incoming request. Similar
to a Task, but accepts a ServiceRequest and returns ServiceResponse
It will usually handle writing an entity to some persistence medium.
*/
type Target interface {
	Apply(ctx context.Context, request *ServiceRequest) (*ServiceResponse, error)
}

type NoOpTarget struct{}

func (t *NoOpTarget) Apply(ctx context.Context, request *ServiceRequest) (*ServiceResponse, error) {
	response := &ServiceResponse{
		ResponseMeta: ResponseMeta{
			StatusCode:       http.StatusOK,
			Proto:            request.RequestMeta.Proto,
			ProtoMajor:       request.RequestMeta.ProtoMajor,
			ProtoMinor:       request.RequestMeta.ProtoMinor,
			Header:           request.Header.Clone(),
			Trailer:          request.Trailer.Clone(),
			TransferEncoding: request.RequestMeta.TransferEncoding,
		},
		Body: request.Body,
	}
	return response, nil
}

func NewNoOpTargetConstructor() TargetConstructor {
	constructor := func(config map[string]any) (Target, error) {
		return &NoOpTarget{}, nil
	}
	return TargetConstructorFunc(constructor)
}
