package entity

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	domainerr "github.com/QueerGlobal/qg-hub/core/entity/error"
	zerolog "github.com/rs/zerolog"
)

type Hub struct {
	APIVersion      string
	Version         string
	ApplicationName string
	services        map[string]*Service
	logger          *zerolog.Logger
}

func NewHub(logger *zerolog.Logger, applicationVersion string) (*Hub, error) {
	hub := &Hub{
		Version:  applicationVersion,
		services: make(map[string]*Service),
		logger:   logger,
	}
	return hub, nil
}

func (hub *Hub) AddService(svc *Service) error {
	serviceKey := strings.ToLower(svc.APIName + "/" + svc.Name)

	hub.services[serviceKey] = svc

	return nil
}

func (hub *Hub) GetService(apiName string, serviceName string) (*Service, bool) {
	serviceKey := strings.ToLower(apiName + "/" + serviceName)

	svc, ok := hub.services[serviceKey]
	if !ok {
		return nil, false
	}
	return svc, true
}

func (hub *Hub) HandleRequest(r *http.Request) (*ServiceResponse, error) {
	request, err := GetRequestFromHttp(r)
	if err != nil {
		hub.logger.Err(err).Str("apiName", request.ApiName).
			Str("serviceName", request.ServiceName).
			Msg("failed to build service request")
		return nil, err
	}

	response, err := hub.executeServiceRequest(r.Context(), request)
	if err != nil {
		hub.logger.Err(err).Str("apiName", request.ApiName).
			Str("serviceName", request.ServiceName).
			Msg("failed to execute service request")
		return nil, err
	}
	return response, nil
}

func (hub *Hub) executeServiceRequest(ctx context.Context, request *ServiceRequest) (*ServiceResponse, error) {
	response := &ServiceResponse{}
	response.ResponseMeta = ResponseMeta{}

	service, ok := hub.GetService(request.ApiName, request.ServiceName)
	if !ok {
		err := fmt.Errorf("service %s not found %w", request.ServiceName, domainerr.ErrServiceNotFound)
		hub.logger.Err(err).Str("apiName", request.ApiName).Str("serviceName", request.ServiceName).Msg("service not found")
		response.ResponseMeta.StatusCode = http.StatusNotFound
		return response, err
	}

	if err := service.DoRequest(ctx, request); err != nil {
		hub.logger.Err(err).Str("apiName", request.ApiName).Str("serviceName", request.ServiceName).Msg("failed to execute service request")
		response.ResponseMeta.StatusCode = http.StatusInternalServerError
		return response, err
	}

	return request.Response, nil
}

func (hub *Hub) SetLogger(l *zerolog.Logger) {
	hub.logger = l
}

func (hub *Hub) GetLogger() *zerolog.Logger {
	return hub.logger
}

func (hub *Hub) GetServices() map[string]*Service {
	return hub.services
}
