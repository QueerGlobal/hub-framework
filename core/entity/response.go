package entity

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type ServiceResponse struct {
	ResponseMeta ResponseMeta
	Body         []byte
}

type ResponseMeta struct {
	OriginalResponse *http.Response
	Status           string
	StatusCode       int
	Proto            string
	ProtoMajor       int
	ProtoMinor       int
	TransferEncoding []string
	Header           http.Header
	Trailer          http.Header
}

func GetEntityFromResponse[T any](r *ServiceResponse) (T, error) {
	var entity T
	err := json.Unmarshal(r.Body, &entity)
	return entity, err
}

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
