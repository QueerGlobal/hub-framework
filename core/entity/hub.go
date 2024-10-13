package entity

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
	"github.com/google/uuid"
	zerolog "github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Hub is the central entity in the system, responsible for managing services and routing HTTP requests.
// It acts as a mediator between incoming HTTP requests and the appropriate services that handle them.
type Hub struct {
	APIVersion      string
	Version         string
	ApplicationName string
	services        map[string]*Service
	logger          *zerolog.Logger
}

// NewHub creates and initializes a new Hub instance.
//
// Parameters:
//   - logger: A pointer to a zerolog.Logger for logging operations.
//   - applicationVersion: A string representing the version of the application.
//
// Returns:
//   - A pointer to the newly created Hub and nil error on success.
//   - nil and an error if initialization fails.
func NewHub(logger *zerolog.Logger, applicationVersion string) (*Hub, error) {
	hub := &Hub{
		Version:  applicationVersion,
		services: make(map[string]*Service),
		logger:   logger,
	}
	return hub, nil
}

// AddService registers a new service with the Hub.
//
// Parameter:
//   - svc: A pointer to the Service to be added.
//
// Returns:
//   - An error if the service couldn't be added, nil otherwise.
func (hub *Hub) AddService(svc *Service) error {
	serviceKey := strings.ToLower(svc.APIName + "/" + svc.Name)
	hub.services[serviceKey] = svc
	return nil
}

// GetService retrieves a service from the Hub by its API name and service name.
//
// Parameters:
//   - apiName: The name of the API.
//   - serviceName: The name of the service.
//
// Returns:
//   - A pointer to the Service and true if found.
//   - nil and false if the service is not found.
func (hub *Hub) GetService(apiName string, serviceName string) (*Service, bool) {
	serviceKey := strings.ToLower(apiName + "/" + serviceName)
	svc, ok := hub.services[serviceKey]
	return svc, ok
}

// HandleRequest is the main entry point for processing HTTP requests.
//
// Parameter:
//   - r: A pointer to the http.Request to be handled.
//
// Returns:
//   - A pointer to ServiceResponse and nil error on success.
//   - nil and an error if request handling fails.
func (hub *Hub) HandleRequest(r *http.Request) (ServiceResponse, error) {
	// Initialize tracer
	tracer := otel.Tracer(hub.ApplicationName)

	// Extract context from headers
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// Start a new span
	ctx, span := tracer.Start(ctx, "HandleRequest")
	defer span.End()

	request, err := GetRequestFromHttp(r)
	if err != nil {
		hub.logger.Err(err).Str("apiName", request.ApiName).
			Str("serviceName", request.ServiceName).
			Msg("failed to build service request")
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build service request")
		return nil, err
	}

	request.ID = uuid.New()

	// Set span attributes
	span.SetAttributes(
		attribute.String("request.id", request.ID.String()),
		attribute.String("api.name", request.ApiName),
		attribute.String("service.name", request.ServiceName),
		attribute.String("http.method", string(r.Method)),
		attribute.String("http.url", r.URL.String()),
	)

	response, err := hub.executeServiceRequest(ctx, request)
	if err != nil {
		hub.logger.Err(err).Str("apiName", request.ApiName).
			Str("serviceName", request.ServiceName).
			Msg("failed to execute service request")
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to execute service request")
		return nil, err
	}

	span.SetStatus(codes.Ok, "request handled successfully")
	return response, nil
}

// executeServiceRequest processes a ServiceRequest by fetching the
// appropriate service and delegating the request handling to that service.
//
// Parameters:
//   - ctx: The context for the request.
//   - request: A pointer to the ServiceRequest to be executed.
//
// Returns:
//   - A pointer to ServiceResponse and nil error on success.
//   - A pointer to ServiceResponse with error details and an error on failure.
func (hub *Hub) executeServiceRequest(ctx context.Context, request ServiceRequest) (ServiceResponse, error) {
	span := trace.SpanFromContext(ctx)

	ctx, localSpan := span.TracerProvider().Tracer(hub.ApplicationName).Start(ctx, request.GetServiceName())
	defer localSpan.End()

	response := &HttpServiceResponse{}
	response.ResponseMeta = &HttpResponseMeta{}

	service, ok := hub.GetService(request.GetAPIName(), request.GetServiceName())
	if !ok {
		err := fmt.Errorf("service %s not found %w", request.GetServiceName(), domainerr.ErrServiceNotFound)
		hub.logger.Err(err).Str("apiName", request.GetAPIName()).Str("serviceName", request.GetServiceName()).Msg("service not found")
		response.ResponseMeta.SetStatusCode(http.StatusNotFound)

		localSpan.RecordError(err)
		localSpan.SetStatus(codes.Error, "service not found")

		return response, err
	}

	if err := service.DoRequest(ctx, request); err != nil {
		hub.logger.Err(err).Str("apiName", request.GetAPIName()).Str("serviceName", request.GetServiceName()).Msg("failed to execute service request")
		response.ResponseMeta.SetStatusCode(http.StatusInternalServerError)

		localSpan.RecordError(err)
		localSpan.SetStatus(codes.Error, "error executing service request")

		return response, err
	}

	return request.GetResponse(), nil
}

// SetLogger sets the logger for the Hub.
//
// Parameter:
//   - l: A pointer to the zerolog.Logger to be used.
func (hub *Hub) SetLogger(l *zerolog.Logger) {
	hub.logger = l
}

// GetLogger returns the current logger used by the Hub.
//
// Returns:
//   - A pointer to the current zerolog.Logger.
func (hub *Hub) GetLogger() *zerolog.Logger {
	return hub.logger
}

// GetServices returns a map of all registered services in the Hub.
//
// Returns:
//   - A map with service keys as strings and Service pointers as values.
func (hub *Hub) GetServices() map[string]*Service {
	return hub.services
}
