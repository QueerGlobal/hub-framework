package remoteforward

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/util"
)

const DefaultClientTimeout = 30 * time.Second

func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}
	return &http.Client{Timeout: timeout}
}

// JoinHostPrefixPath builds a forward URL without a double slash between prefix and path.
func JoinHostPrefixPath(host, pathPrefix, requestPath string) string {
	host = strings.TrimSuffix(host, "/")
	prefix := pathPrefix
	path := requestPath
	if prefix != "" && strings.HasSuffix(prefix, "/") && strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	return host + prefix + path
}

func StatusCodeError(statusCode int) error {
	if statusCode >= 500 {
		return fmt.Errorf("remote server returned status %d", statusCode)
	}
	if statusCode >= 400 {
		return fmt.Errorf("%s: remote server returned status %d", util.UnrecoverableErrorMsg, statusCode)
	}
	return nil
}

func HTTPToServiceResponse(resp *http.Response, body []byte) *entity.HttpServiceResponse {
	return &entity.HttpServiceResponse{
		ResponseMeta: &entity.HttpResponseMeta{
			OriginalResponse: resp,
			Status:           resp.Status,
			StatusCode:       resp.StatusCode,
			Proto:            resp.Proto,
			ProtoMajor:       resp.ProtoMajor,
			ProtoMinor:       resp.ProtoMinor,
			TransferEncoding: resp.TransferEncoding,
			Header:           resp.Header,
			Trailer:          resp.Trailer,
		},
		Body: body,
	}
}

func ReadAndValidateResponse(resp *http.Response) (*entity.HttpServiceResponse, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", util.UnrecoverableErrorMsg, err)
	}

	if err := StatusCodeError(resp.StatusCode); err != nil {
		return nil, err
	}

	return HTTPToServiceResponse(resp, body), nil
}
