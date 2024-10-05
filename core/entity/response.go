package entity

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// ServiceResponseInterface represents the interface for a standardized response structure.
type ServiceResponse interface {
	GetResponseMeta() ResponseMeta
	SetResponseMeta(meta ResponseMeta)
	GetBody() []byte
	SetBody(body []byte)
}

// ResponseMetaInterface represents the interface for response metadata.
type ResponseMeta interface {
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
type HttpServiceResponse struct {
	ResponseMeta ResponseMeta // Metadata about the response
	Body         []byte       // Raw body of the response
}

// ResponseMeta contains metadata about the HTTP response.
type HttpResponseMeta struct {
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
func GetEntityFromResponse[T any](r ServiceResponse) (T, error) {
	var entity T
	err := json.Unmarshal(r.GetBody(), &entity)
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
func GetResponseFromHttp(r *http.Response) (*HttpServiceResponse, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	response := HttpServiceResponse{
		ResponseMeta: &HttpResponseMeta{
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
func GetHttpFromResponse(r ServiceResponse) (*http.Response, error) {
	responseBody := io.NopCloser(bytes.NewReader(r.GetBody()))
	contentLength := int64(len(r.GetBody()))

	response := http.Response{
		Status:           r.GetResponseMeta().GetStatus(),
		StatusCode:       r.GetResponseMeta().GetStatusCode(),
		Proto:            r.GetResponseMeta().GetProto(),
		ProtoMajor:       r.GetResponseMeta().GetProtoMajor(),
		ProtoMinor:       r.GetResponseMeta().GetProtoMinor(),
		Header:           r.GetResponseMeta().GetHeader(),
		Body:             responseBody,
		ContentLength:    contentLength,
		TransferEncoding: r.GetResponseMeta().GetTransferEncoding(),
		Close:            true,
		Uncompressed:     false,
		Trailer:          r.GetResponseMeta().GetTrailer(),
	}
	return &response, nil
}

// ServiceResponse methods

func (sr *HttpServiceResponse) GetResponseMeta() ResponseMeta {
	return sr.ResponseMeta
}

func (sr *HttpServiceResponse) SetResponseMeta(meta ResponseMeta) {
	sr.ResponseMeta = meta
}

func (sr *HttpServiceResponse) GetBody() []byte {
	return sr.Body
}

func (sr *HttpServiceResponse) SetBody(body []byte) {
	sr.Body = body
}

// ResponseMeta methods

func (rm *HttpResponseMeta) GetOriginalResponse() *http.Response {
	return rm.OriginalResponse
}

func (rm *HttpResponseMeta) SetOriginalResponse(resp *http.Response) {
	rm.OriginalResponse = resp
}

func (rm *HttpResponseMeta) GetStatus() string {
	return rm.Status
}

func (rm *HttpResponseMeta) SetStatus(status string) {
	rm.Status = status
}

func (rm *HttpResponseMeta) GetStatusCode() int {
	return rm.StatusCode
}

func (rm *HttpResponseMeta) SetStatusCode(code int) {
	rm.StatusCode = code
}

func (rm *HttpResponseMeta) GetProto() string {
	return rm.Proto
}

func (rm *HttpResponseMeta) SetProto(proto string) {
	rm.Proto = proto
}

func (rm *HttpResponseMeta) GetProtoMajor() int {
	return rm.ProtoMajor
}

func (rm *HttpResponseMeta) SetProtoMajor(major int) {
	rm.ProtoMajor = major
}

func (rm *HttpResponseMeta) GetProtoMinor() int {
	return rm.ProtoMinor
}

func (rm *HttpResponseMeta) SetProtoMinor(minor int) {
	rm.ProtoMinor = minor
}

func (rm *HttpResponseMeta) GetTransferEncoding() []string {
	return rm.TransferEncoding
}

func (rm *HttpResponseMeta) SetTransferEncoding(encoding []string) {
	rm.TransferEncoding = encoding
}

func (rm *HttpResponseMeta) GetHeader() http.Header {
	return rm.Header
}

func (rm *HttpResponseMeta) SetHeader(header http.Header) {
	rm.Header = header
}

func (rm *HttpResponseMeta) GetTrailer() http.Header {
	return rm.Trailer
}

func (rm *HttpResponseMeta) SetTrailer(trailer http.Header) {
	rm.Trailer = trailer
}
