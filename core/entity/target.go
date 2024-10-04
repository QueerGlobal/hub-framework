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
	Apply(ctx context.Context, request ServiceRequest) (ServiceResponseInterface, error)
}

type NoOpTarget struct{}

func (t *NoOpTarget) Apply(ctx context.Context, request ServiceRequest) (ServiceResponseInterface, error) {
	response := &ServiceResponse{
		ResponseMeta: ResponseMeta{
			StatusCode:       http.StatusOK,
			Proto:            request.GetRequestMeta().GetProto(),
			ProtoMajor:       request.GetRequestMeta().GetProtoMajor(),
			ProtoMinor:       request.GetRequestMeta().GetProtoMinor(),
			Header:           request.GetHeader().Clone(),
			Trailer:          request.GetTrailer().Clone(),
			TransferEncoding: request.GetRequestMeta().GetTransferEncoding(),
		},
		Body: request.GetBody(),
	}
	return response, nil
}

func NewNoOpTargetConstructor() TargetConstructor {
	constructor := func(config map[string]any) (Target, error) {
		return &NoOpTarget{}, nil
	}
	return TargetConstructorFunc(constructor)
}
