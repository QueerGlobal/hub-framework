package target

import (
	"context"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

// Noop is a target that does nothing
type Noop struct{}

// NewNoop creates a new Noop target
func NewNoop(config map[string]interface{}) (entity.Target, error) {
	return &Noop{}, nil
}

// Apply implements the Target interface but does nothing
func (n *Noop) Apply(ctx context.Context, req entity.ServiceRequest) (entity.ServiceResponse, error) {
	// Return a concrete type that implements ServiceResponse
	return &entity.HttpServiceResponse{
		Body: req.GetBody(),
		ResponseMeta: &entity.HttpResponseMeta{
			StatusCode:       200,
			Proto:            req.GetRequestMeta().GetProto(),
			ProtoMajor:       req.GetRequestMeta().GetProtoMajor(),
			ProtoMinor:       req.GetRequestMeta().GetProtoMinor(),
			TransferEncoding: req.GetRequestMeta().GetTransferEncoding(),
			Header:           req.GetHeader(),
			Trailer:          req.GetTrailer(),
		},
	}, nil
}
