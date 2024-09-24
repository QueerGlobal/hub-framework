package entity

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

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
