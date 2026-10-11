package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/QueerGlobal/hub-framework/service/remoteforward"
	"github.com/QueerGlobal/hub-framework/util"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ForwardingService struct {
	Host       string
	PathPrefix string
	name       string
	backoff    *util.Backoff
	httpClient *http.Client
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
			backoffConfig.InitialDelay = time.Duration(initialDelay * float64(time.Second))
		}
		if maxDelay, ok := backoff["maxDelay"].(float64); ok {
			backoffConfig.MaxDelay = time.Duration(maxDelay * float64(time.Second))
		}
		if multiplier, ok := backoff["multiplier"].(float64); ok {
			backoffConfig.Multiplier = multiplier
		}
		if maxRetries, ok := backoff["maxRetries"].(float64); ok {
			backoffConfig.MaxRetries = int(maxRetries)
		}
	}

	var clientTimeout time.Duration
	if timeoutSeconds, ok := config["clientTimeout"].(float64); ok {
		clientTimeout = time.Duration(timeoutSeconds * float64(time.Second))
	}

	backoff := util.NewBackoff(backoffConfig)
	svc.backoff = backoff
	svc.httpClient = remoteforward.NewHTTPClient(clientTimeout)

	return &svc, nil
}

func (fs *ForwardingService) Apply(ctx context.Context, request entity.ServiceRequest) error {
	err := fs.backoff.ExecuteWithBackoff(func() error {
		response, err := fs.forwardRequest(ctx, request)
		if err != nil {
			return err
		}

		responseObj, err := remoteforward.ReadAndValidateResponse(response)
		if err != nil {
			return err
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

func (fs *ForwardingService) forwardRequest(ctx context.Context, request entity.ServiceRequest) (*http.Response, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	requestPath := ""
	if request.GetURL() != nil {
		requestPath = request.GetURL().Path
	}

	urlString := remoteforward.JoinHostPrefixPath(fs.Host, fs.PathPrefix, requestPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlString, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Inject the trace context into the request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := fs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %w", err)
	}

	return resp, nil
}
