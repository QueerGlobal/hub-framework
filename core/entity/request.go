package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ServiceRequestInterface represents the interface for a standardized request structure.
type ServiceRequest interface {
	GetID() uuid.UUID
	GetAPIName() string
	GetServiceName() string
	GetMethod() HTTPMethod
	GetURL() *url.URL
	GetInternalPath() string
	GetBody() []byte
	SetBody(body []byte)
	GetForm() *url.Values
	SetForm(form *url.Values)
	GetPostForm() *url.Values
	SetPostForm(form *url.Values)
	GetMultipart() MultipartDataInterface
	SetMultipart(data MultipartDataInterface)
	GetResponse() ServiceResponse
	SetResponse(response ServiceResponse)
	GetRequestMeta() RequestMetaInterface
	SetRequestMeta(meta RequestMetaInterface)
	GetHeader() http.Header
	SetHeader(header http.Header)
	GetTrailer() http.Header
	SetTrailer(trailer http.Header)
	InjectTraceFromContext(ctx context.Context)
}

// MultipartDataInterface represents the interface for multipart form data.
type MultipartDataInterface interface {
	GetValue() map[string][]string
	SetValue(value map[string][]string)
	GetFileData() map[string][]byte
	SetFileData(fileData map[string][]byte)
}

// RequestMetaInterface represents the interface for request metadata.
type RequestMetaInterface interface {
	GetOriginalRequest() *http.Request
	SetOriginalRequest(req *http.Request)
	GetParams() map[string]string
	SetParams(params map[string]string)
	GetProto() string
	SetProto(proto string)
	GetProtoMajor() int
	SetProtoMajor(major int)
	GetProtoMinor() int
	SetProtoMinor(minor int)
	GetContentLength() int64
	SetContentLength(length int64)
	GetTransferEncoding() []string
	SetTransferEncoding(encoding []string)
	GetHost() string
	SetHost(host string)
	GetRemoteAddr() string
	SetRemoteAddr(addr string)
	GetRequestURI() string
	SetRequestURI(uri string)
}

// ServiceRequest represents a standardized request structure used within the service.
type HTTPServiceRequest struct {
	ID           uuid.UUID        // Unique identifier for the request
	ApiName      string           // Name of the API being called
	ServiceName  string           // Name of the specific service within the API
	Method       HTTPMethod       // HTTP method of the request
	URL          *url.URL         // Full URL of the request
	InternalPath string           // Internal path after removing prefixes
	Body         []byte           // Raw body of the request
	Form         *url.Values      // URL-encoded form data
	PostForm     *url.Values      // Posted form data
	Multipart    *MultipartData   // Multipart form data, including file uploads
	Response     *ServiceResponse // Associated response (if any)
	RequestMeta  RequestMeta      // Additional metadata about the request
	Header       http.Header      // HTTP headers
	Trailer      http.Header      // HTTP trailers
}

// MultipartData holds both regular form values and file data for multipart requests.
type MultipartData struct {
	Value    map[string][]string // Regular form values
	FileData map[string][]byte   // File data, keyed by field name
}

// RequestMeta contains additional metadata about the original HTTP request.
type RequestMeta struct {
	OriginalRequest  *http.Request     // The original http.Request
	Params           map[string]string // Additional parameters (e.g., from router)
	Proto            string            // Protocol version
	ProtoMajor       int               // Major protocol version
	ProtoMinor       int               // Minor protocol version
	ContentLength    int64             // Length of the request body
	TransferEncoding []string          // Transfer encodings
	Host             string            // Requested host
	RemoteAddr       string            // Remote address of the client
	RequestURI       string            // Unmodified request-target of the Request-Line
}

// headerCarrier adapts http.Header to implement the TextMapCarrier interface
type headerCarrier http.Header

func (hc headerCarrier) Set(key, value string) {
	http.Header(hc).Set(key, value)
}

func (hc headerCarrier) Get(key string) string {
	return http.Header(hc).Get(key)
}

func (hc headerCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range hc {
		keys = append(keys, k)
	}
	return keys
}

// GetEntityFromRequest unmarshals the request body into a given entity type.
//
// Parameters:
//   - r: A pointer to a ServiceRequest containing the request data.
//
// Returns:
//   - T: The unmarshaled entity of type T.
//   - error: An error if unmarshaling fails, nil otherwise.
func GetEntityFromRequest[T any](r *HTTPServiceRequest) (T, error) {
	var entity T
	if err := json.Unmarshal(r.Body, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

// GetRequestFromHttp converts a standard http.Request to our custom ServiceRequest.
//
// Parameters:
//   - r: A pointer to an http.Request to be converted.
//
// Returns:
//   - *ServiceRequest: A pointer to the converted ServiceRequest.
//   - error: An error if conversion fails, nil otherwise.
func GetRequestFromHttp(r *http.Request) (*HTTPServiceRequest, error) {
	if r == nil {
		return nil, domainerr.ErrEmptyInput
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	multipartData := MultipartData{}

	if r.MultipartForm != nil {
		multipartData.Value = r.MultipartForm.Value
		multipartData.FileData = make(map[string][]byte)

		for fieldName, fileHeaders := range r.MultipartForm.File {
			for _, fileHeader := range fileHeaders {
				file, err := fileHeader.Open()
				if err != nil {
					return nil, fmt.Errorf("error opening file %s: %w", fieldName, err)
				}
				defer file.Close()

				fileBytes, err := io.ReadAll(file)
				if err != nil {
					return nil, fmt.Errorf("error reading file %s: %w", fieldName, err)
				}

				multipartData.FileData[fieldName] = fileBytes
			}
		}
	}

	internalPath := r.URL.Path

	// Remove the /internal/call prefix if it exists
	if strings.HasPrefix(internalPath, "/internal/call") {
		internalPath = strings.Replace(internalPath, "/internal/call", "", 1)
	}

	// Split the internal path
	pathSegments := strings.Split(strings.Trim(internalPath, "/"), "/")

	var apiName, serviceName string

	if len(pathSegments) >= 2 {
		apiName = pathSegments[0]
		serviceName = pathSegments[1]
	} else if len(pathSegments) == 1 {
		// Handle case where only one segment is present
		apiName = pathSegments[0]
	}
	// If no segments, both apiName and serviceName remain empty

	httpMethod, err := StringToHTTPMethod(r.Method)
	if err != nil {
		return nil, err
	}

	response := HTTPServiceRequest{
		Method:       httpMethod,
		ApiName:      apiName,
		ServiceName:  serviceName,
		URL:          r.URL,
		Body:         body,
		Form:         &r.Form,
		PostForm:     &r.PostForm,
		Header:       r.Header,
		Trailer:      r.Trailer,
		InternalPath: internalPath,
		RequestMeta: RequestMeta{
			OriginalRequest:  r,
			Proto:            r.Proto,
			ProtoMajor:       r.ProtoMajor,
			ProtoMinor:       r.ProtoMinor,
			TransferEncoding: r.TransferEncoding,
		},
	}

	return &response, nil
}

// ServiceRequest methods

func (sr *HTTPServiceRequest) GetAPIName() string {
	return sr.ApiName
}

func (sr *HTTPServiceRequest) SetAPIName(name string) {
	sr.ApiName = name
}

func (sr *HTTPServiceRequest) GetServiceName() string {
	return sr.ServiceName
}

func (sr *HTTPServiceRequest) SetServiceName(name string) {
	sr.ServiceName = name
}

func (sr *HTTPServiceRequest) GetMethod() HTTPMethod {
	return sr.Method
}

func (sr *HTTPServiceRequest) SetMethod(method HTTPMethod) {
	sr.Method = method
}

func (sr *HTTPServiceRequest) GetURL() *url.URL {
	return sr.URL
}

func (sr *HTTPServiceRequest) SetURL(url *url.URL) {
	sr.URL = url
}

func (sr *HTTPServiceRequest) GetInternalPath() string {
	return sr.InternalPath
}

func (sr *HTTPServiceRequest) SetInternalPath(path string) {
	sr.InternalPath = path
}

func (sr *HTTPServiceRequest) GetBody() []byte {
	return sr.Body
}

func (sr *HTTPServiceRequest) SetBody(body []byte) {
	sr.Body = body
}

func (sr *HTTPServiceRequest) GetForm() *url.Values {
	return sr.Form
}

func (sr *HTTPServiceRequest) SetForm(form *url.Values) {
	sr.Form = form
}

func (sr *HTTPServiceRequest) GetPostForm() *url.Values {
	return sr.PostForm
}

func (sr *HTTPServiceRequest) SetPostForm(form *url.Values) {
	sr.PostForm = form
}

func (sr *HTTPServiceRequest) GetMultipart() MultipartDataInterface {
	return sr.Multipart
}

func (sr *HTTPServiceRequest) SetMultipart(data MultipartDataInterface) {
	if multipartData, ok := data.(*MultipartData); ok {
		sr.Multipart = multipartData
	}
}

func (sr *HTTPServiceRequest) GetResponse() ServiceResponse {
	return *sr.Response
}

func (sr *HTTPServiceRequest) SetResponse(response ServiceResponse) {
	sr.Response = &response
}

func (sr *HTTPServiceRequest) GetRequestMeta() RequestMetaInterface {
	return &sr.RequestMeta
}

func (sr *HTTPServiceRequest) SetRequestMeta(meta RequestMetaInterface) {
	if requestMeta, ok := meta.(*RequestMeta); ok {
		sr.RequestMeta = *requestMeta
	}
}

func (sr *HTTPServiceRequest) GetHeader() http.Header {
	return sr.Header
}

func (sr *HTTPServiceRequest) SetHeader(header http.Header) {
	sr.Header = header
}

func (sr *HTTPServiceRequest) GetTrailer() http.Header {
	return sr.Trailer
}

func (sr *HTTPServiceRequest) SetTrailer(trailer http.Header) {
	sr.Trailer = trailer
}

func (sr *HTTPServiceRequest) GetID() uuid.UUID {
	return sr.ID
}

func (sr *HTTPServiceRequest) SetID(id uuid.UUID) {
	sr.ID = id
}

// MultipartData methods

func (md *MultipartData) GetValue() map[string][]string {
	return md.Value
}

func (md *MultipartData) SetValue(value map[string][]string) {
	md.Value = value
}

func (md *MultipartData) GetFileData() map[string][]byte {
	return md.FileData
}

func (md *MultipartData) SetFileData(fileData map[string][]byte) {
	md.FileData = fileData
}

// RequestMeta methods

func (rm *RequestMeta) GetOriginalRequest() *http.Request {
	return rm.OriginalRequest
}

func (rm *RequestMeta) SetOriginalRequest(req *http.Request) {
	rm.OriginalRequest = req
}

func (rm *RequestMeta) GetParams() map[string]string {
	return rm.Params
}

func (rm *RequestMeta) SetParams(params map[string]string) {
	rm.Params = params
}

func (rm *RequestMeta) GetProto() string {
	return rm.Proto
}

func (rm *RequestMeta) SetProto(proto string) {
	rm.Proto = proto
}

func (rm *RequestMeta) GetProtoMajor() int {
	return rm.ProtoMajor
}

func (rm *RequestMeta) SetProtoMajor(major int) {
	rm.ProtoMajor = major
}

func (rm *RequestMeta) GetProtoMinor() int {
	return rm.ProtoMinor
}

func (rm *RequestMeta) SetProtoMinor(minor int) {
	rm.ProtoMinor = minor
}

func (rm *RequestMeta) GetContentLength() int64 {
	return rm.ContentLength
}

func (rm *RequestMeta) SetContentLength(length int64) {
	rm.ContentLength = length
}

func (rm *RequestMeta) GetTransferEncoding() []string {
	return rm.TransferEncoding
}

func (rm *RequestMeta) SetTransferEncoding(encoding []string) {
	rm.TransferEncoding = encoding
}

func (rm *RequestMeta) GetHost() string {
	return rm.Host
}

func (rm *RequestMeta) SetHost(host string) {
	rm.Host = host
}

func (rm *RequestMeta) GetRemoteAddr() string {
	return rm.RemoteAddr
}

func (rm *RequestMeta) SetRemoteAddr(addr string) {
	rm.RemoteAddr = addr
}

func (rm *RequestMeta) GetRequestURI() string {
	return rm.RequestURI
}

func (rm *RequestMeta) SetRequestURI(uri string) {
	rm.RequestURI = uri
}

// InjectTrace injects OpenTelemetry trace information into the request headers.
// It takes a context and updates the request's header with the trace context.
func (r *HTTPServiceRequest) InjectTraceFromContext(ctx context.Context) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))
}

// StartSpan starts a new span for the request and injects the trace context into the request headers.
// It returns the new context with the span and the span itself.
func (r *HTTPServiceRequest) StartSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	tracer := otel.Tracer("http-service-request")
	ctx, span := tracer.Start(ctx, spanName)

	// Set span attributes
	span.SetAttributes(
		attribute.String("request.id", r.ID.String()),
		attribute.String("api.name", r.ApiName),
		attribute.String("service.name", r.ServiceName),
		attribute.String("http.method", string(r.Method)),
		attribute.String("http.url", r.URL.String()),
	)

	// Inject the trace context into the request headers
	r.InjectTraceFromContext(ctx)

	return ctx, span
}

// EndSpan ends the span and injects the trace context into the request headers.
func (r *HTTPServiceRequest) EndSpan(ctx context.Context, span trace.Span, status codes.Code, message string) {
	r.InjectTraceFromContext(ctx)
	span.SetStatus(status, message)
	span.End()
}
