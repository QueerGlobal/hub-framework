package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

// TraceTask is a task that initiates an OpenTelemetry trace
type TraceTask struct {
	tracer trace.Tracer
}

// NewTraceTask creates a new TraceTask
func NewTraceTask() *TraceTask {
	return &TraceTask{
		tracer: otel.Tracer("telemetry"),
	}
}

// Name returns the name of the task
func (t *TraceTask) Name() string {
	return "OpenTelemetry Trace"
}

// Apply initiates a new OpenTelemetry trace
func (t *TraceTask) Apply(ctx context.Context, request *entity.HTTPServiceRequest) error {
	// Create a new span
	ctx = otel.GetTextMapPropagator().Extract(request.Context(), request.Headers)
	ctx, span := t.tracer.Start(ctx, "IncomingRequest")
	defer span.End()

	// Create a new Trace object if it doesn't exist
	if request.Trace == nil {
		request.Trace = &entity.Trace{}
	}

	// Update the Trace object with span information
	request.Trace.TraceID = span.SpanContext().TraceID().String()
	request.Trace.SpanID = span.SpanContext().SpanID().String()
	request.Trace.TraceFlags = byte(span.SpanContext().TraceFlags())

	// Add any relevant attributes to the span
	span.SetAttributes(
		attribute.String("request.id", request.ID.String()),
		attribute.String("api.name", request.ApiName),
		attribute.String("service.name", request.ServiceName),
		// Add more attributes as needed
	)

	// Inject the span context back into the request headers
	otel.GetTextMapPropagator().Inject(ctx, request.GetHeader())

	return nil
}

// TODO: INitate trace in request handler in hub
// TODO: Add trace to response
// TODO: Add span to each task call from hub, with task name
// TODO: Add span to each db call from hub, with query
// TODO: Add span to each redis call from hub, with query
// TODO: Add span to each pubsub call from hub, with topic
// TODO: Add span to each external call from hub, with url
// TODO: Add span to each db call from hub, with query
