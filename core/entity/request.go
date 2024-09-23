package entity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
)

// ServiceRequest represents a standardized request structure used within the service.
type ServiceRequest struct {
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

// GetEntityFromRequest unmarshals the request body into a given entity type.
//
// Parameters:
//   - r: A pointer to a ServiceRequest containing the request data.
//
// Returns:
//   - T: The unmarshaled entity of type T.
//   - error: An error if unmarshaling fails, nil otherwise.
func GetEntityFromRequest[T any](r *ServiceRequest) (T, error) {
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
func GetRequestFromHttp(r *http.Request) (*ServiceRequest, error) {
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

	response := ServiceRequest{
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
