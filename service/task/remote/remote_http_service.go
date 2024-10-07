package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/util"
)

type ForwardingService struct {
	Host       string
	PathPrefix string
	name       string
	backoff    *util.Backoff
}

func NewForwardingService(config map[string]interface{}) (entity.Task, error) {
	svc := ForwardingService{}

	if host, ok := config["host"].(string); ok {
		svc.Host = host
	}

	if pathPrefix, ok := config["pathprefix"].(string); ok {
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
	}

	var backoffConfig util.BackoffConfig
	if backoff, ok := config["backoff"].(map[string]interface{}); ok {
		if initialDelay, ok := backoff["initialDelay"].(float64); ok {
			backoffConfig.InitialDelay = time.Duration(initialDelay) * time.Second
		}
		if maxDelay, ok := backoff["maxDelay"].(float64); ok {
			backoffConfig.MaxDelay = time.Duration(maxDelay) * time.Second
		}
		if multiplier, ok := backoff["multiplier"].(float64); ok {
			backoffConfig.Multiplier = multiplier
		}
		if maxRetries, ok := backoff["maxRetries"].(float64); ok {
			backoffConfig.MaxRetries = int(maxRetries)
		}
	}

	backoff := util.NewBackoff(backoffConfig)
	svc.backoff = backoff

	return &svc, nil
}

func (fs *ForwardingService) Apply(ctx context.Context, request entity.ServiceRequest) error {
	var serviceResponse entity.ServiceRequest

	err := fs.backoff.ExecuteWithBackoff(func() error {
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

		responseObj, err := entity.GetResponseFromHttp(response)
		if err != nil {
			return fmt.Errorf("error getting response from http: %w", err)
		}

		request.SetResponse(responseObj)

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (fs *ForwardingService) Name() string {
	return fs.name
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
