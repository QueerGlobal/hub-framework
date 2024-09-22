package entity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	domainerr "github.com/QueerGlobal/qg-hub/core/entity/error"
)

type ServiceRequest struct {
	ApiName      string
	ServiceName  string
	Method       HTTPMethod
	URL          *url.URL
	InternalPath string
	Body         []byte
	Form         *url.Values
	PostForm     *url.Values
	Multipart    *MultipartData
	Response     *ServiceResponse
	RequestMeta  RequestMeta
	Header       http.Header
	Trailer      http.Header
}

type MultipartData struct {
	Value    map[string][]string
	FileData map[string][]byte
}

type RequestMeta struct {
	OriginalRequest  *http.Request
	Params           map[string]string
	Proto            string
	ProtoMajor       int
	ProtoMinor       int
	ContentLength    int64
	TransferEncoding []string
	Host             string
	RemoteAddr       string
	RequestURI       string
}

func GetEntityFromRequest[T any](r *ServiceRequest) (T, error) {
	var entity T
	if err := json.Unmarshal(r.Body, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

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

	//remove the /internal/call prefix if it exists
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
