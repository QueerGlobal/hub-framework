package entity

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// ServiceResponseInterface represents the interface for a standardized response structure.
type ServiceResponseInterface interface {
	GetResponseMeta() ResponseMetaInterface
	SetResponseMeta(meta ResponseMetaInterface)
	GetBody() []byte
	SetBody(body []byte)
}

// ResponseMetaInterface represents the interface for response metadata.
type ResponseMetaInterface interface {
	GetOriginalResponse() *http.Response
	GetStatus() string
	SetStatus(status string)
	GetStatusCode() int
	SetStatusCode(code int)
	GetProto() string
	SetProto(proto string)
	GetProtoMajor() int
	SetProtoMajor(major int)
	GetProtoMinor() int
	SetProtoMinor(minor int)
	GetTransferEncoding() []string
	SetTransferEncoding(encoding []string)
	GetHeader() http.Header
	SetHeader(header http.Header)
	GetTrailer() http.Header
	SetTrailer(trailer http.Header)
}

// ServiceResponse represents a standardized response structure used within the service.
type ServiceResponse struct {
	ResponseMeta ResponseMeta // Metadata about the response
	Body         []byte       // Raw body of the response
}

// ResponseMeta contains metadata about the HTTP response.
type ResponseMeta struct {
	OriginalResponse *http.Response // The original http.Response
	Status           string         // Status line of the response
	StatusCode       int            // Status code of the response
	Proto            string         // Protocol version
	ProtoMajor       int            // Major protocol version
	ProtoMinor       int            // Minor protocol version
	TransferEncoding []string       // Transfer encodings
	Header           http.Header    // HTTP headers
	Trailer          http.Header    // HTTP trailers
}

// GetEntityFromResponse unmarshals the response body into a given entity type.
//
// Parameters:
//   - r: A pointer to a ServiceResponse containing the response data.
//
// Returns:
//   - T: The unmarshaled entity of type T.
//   - error: An error if unmarshaling fails, nil otherwise.
func GetEntityFromResponse[T any](r *ServiceResponse) (T, error) {
	var entity T
	err := json.Unmarshal(r.Body, &entity)
	return entity, err
}

// GetResponseFromHttp converts a standard http.Response to our custom ServiceResponse.
//
// Parameters:
//   - r: A pointer to an http.Response to be converted.
//
// Returns:
//   - *ServiceResponse: A pointer to the converted ServiceResponse.
//   - error: An error if conversion fails, nil otherwise.
func GetResponseFromHttp(r *http.Response) (*ServiceResponse, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	response := ServiceResponse{
		ResponseMeta: ResponseMeta{
			OriginalResponse: r,
			Status:           r.Status,
			StatusCode:       r.StatusCode,
			Proto:            r.Proto,
			ProtoMajor:       r.ProtoMajor,
			ProtoMinor:       r.ProtoMinor,
			TransferEncoding: r.TransferEncoding,
			Header:           r.Header,
			Trailer:          r.Trailer,
		},
		Body: body,
	}

	return &response, nil
}

// GetHttpFromResponse converts our custom ServiceResponse back to a standard http.Response.
//
// Parameters:
//   - r: A pointer to a ServiceResponse to be converted.
//
// Returns:
//   - *http.Response: A pointer to the converted http.Response.
//   - error: An error if conversion fails, nil otherwise.
func GetHttpFromResponse(r *ServiceResponse) (*http.Response, error) {
	responseBody := io.NopCloser(bytes.NewReader(r.Body))
	contentLength := int64(len(r.Body))

	response := http.Response{
		Status:           r.ResponseMeta.Status,
		StatusCode:       r.ResponseMeta.StatusCode,
		Proto:            r.ResponseMeta.Proto,
		ProtoMajor:       r.ResponseMeta.ProtoMajor,
		ProtoMinor:       r.ResponseMeta.ProtoMinor,
		Header:           r.ResponseMeta.Header,
		Body:             responseBody,
		ContentLength:    contentLength,
		TransferEncoding: r.ResponseMeta.TransferEncoding,
		Close:            true,
		Uncompressed:     false,
		Trailer:          r.ResponseMeta.Trailer,
	}
	return &response, nil
}

// ServiceResponse methods

func (sr *ServiceResponse) GetResponseMeta() ResponseMetaInterface {
	return &sr.ResponseMeta
}

func (sr *ServiceResponse) SetResponseMeta(meta ResponseMetaInterface) {
	if responseMeta, ok := meta.(*ResponseMeta); ok {
		sr.ResponseMeta = *responseMeta
	}
}

func (sr *ServiceResponse) GetBody() []byte {
	return sr.Body
}

func (sr *ServiceResponse) SetBody(body []byte) {
	sr.Body = body
}

// ResponseMeta methods

func (rm *ResponseMeta) GetOriginalResponse() *http.Response {
	return rm.OriginalResponse
}

func (rm *ResponseMeta) SetOriginalResponse(resp *http.Response) {
	rm.OriginalResponse = resp
}

func (rm *ResponseMeta) GetStatus() string {
	return rm.Status
}

func (rm *ResponseMeta) SetStatus(status string) {
	rm.Status = status
}

func (rm *ResponseMeta) GetStatusCode() int {
	return rm.StatusCode
}

func (rm *ResponseMeta) SetStatusCode(code int) {
	rm.StatusCode = code
}

func (rm *ResponseMeta) GetProto() string {
	return rm.Proto
}

func (rm *ResponseMeta) SetProto(proto string) {
	rm.Proto = proto
}

func (rm *ResponseMeta) GetProtoMajor() int {
	return rm.ProtoMajor
}

func (rm *ResponseMeta) SetProtoMajor(major int) {
	rm.ProtoMajor = major
}

func (rm *ResponseMeta) GetProtoMinor() int {
	return rm.ProtoMinor
}

func (rm *ResponseMeta) SetProtoMinor(minor int) {
	rm.ProtoMinor = minor
}

func (rm *ResponseMeta) GetTransferEncoding() []string {
	return rm.TransferEncoding
}

func (rm *ResponseMeta) SetTransferEncoding(encoding []string) {
	rm.TransferEncoding = encoding
}

func (rm *ResponseMeta) GetHeader() http.Header {
	return rm.Header
}

func (rm *ResponseMeta) SetHeader(header http.Header) {
	rm.Header = header
}

func (rm *ResponseMeta) GetTrailer() http.Header {
	return rm.Trailer
}

func (rm *ResponseMeta) SetTrailer(trailer http.Header) {
	rm.Trailer = trailer
}
