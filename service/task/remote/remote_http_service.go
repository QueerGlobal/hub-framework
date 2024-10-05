package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/util"
)

type ForwardingService struct {
	Host       string
	PathPrefix string
	backoff    *util.Backoff
}

type ForwardingServiceConfig struct {
	Host          string
	PathPrefix    string
	BackoffConfig util.BackoffConfig
}

func NewForwardingService(config interface{}) ForwardingService {
	svc := ForwardingService{}

	cfg := config.(ForwardingServiceConfig)

	svc.Host = cfg.Host

	pathPrefix := cfg.PathPrefix

	if len(pathPrefix) > 0 {
		// Prepend "/" if it doesn't exist
		if !strings.HasPrefix(pathPrefix, "/") {
			pathPrefix = "/" + pathPrefix
		}

		// Append "/" if it doesn't exist
		if !strings.HasSuffix(pathPrefix, "/") {
			pathPrefix = pathPrefix + "/"
		}
	}

	svc.PathPrefix = pathPrefix

	backoff := util.NewBackoff(cfg.BackoffConfig)
	svc.backoff = backoff

	return svc
}

func (fs *ForwardingService) Apply(request entity.ServiceRequest) (entity.ServiceRequest, error) {
	var serviceResponse entity.ServiceRequest
	var err error

	err = fs.backoff.ExecuteWithBackoff(func() error {
		response, err := fs.forwardRequest(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("%s: %w", util.UnrecoverableErrorMsg, err)
		}

		if err := json.Unmarshal(body, &serviceResponse); err != nil {
			return fmt.Errorf("error unmarshaling response: %s: %w", util.UnrecoverableErrorMsg, err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return serviceResponse, nil
}

func (fs *ForwardingService) forwardRequest(request entity.ServiceRequest) (*http.Response, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	requestPath := ""
	if request.GetURL() != nil {
		requestPath = request.GetURL().Path
	}

	urlString := fs.Host + fs.PathPrefix + requestPath

	req, err := http.NewRequest("POST", urlString, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %w", err)
	}

	return resp, nil
}
