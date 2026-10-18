package remotehttpservice

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
	"github.com/rs/zerolog"
)

type ForwardingService struct {
	Host       string
	PathPrefix string
	backoff    *util.Backoff
	httpClient *http.Client
	forward    remoteforward.ForwardConfig
}

type ForwardingServiceConfig struct {
	Host                 string
	PathPrefix           string
	BackoffConfig        util.BackoffConfig
	ClientTimeout        time.Duration
	MaxResponseBodyBytes int64
	Logger               *zerolog.Logger
	Sentry               remoteforward.SentryConfig
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
	svc.httpClient = remoteforward.NewHTTPClient(cfg.ClientTimeout)
	settings := remoteforward.ForwardSettings{
		MaxResponseBodyBytes: cfg.MaxResponseBodyBytes,
		Logger:               cfg.Logger,
		Sentry:               cfg.Sentry,
		UpstreamHost:         cfg.Host,
	}
	forwardCfg, err := remoteforward.NewForwardConfig(settings)
	if err != nil {
		logger := cfg.Logger
		if logger == nil {
			defaultLog := remoteforward.DefaultLogger()
			logger = &defaultLog
		}
		logger.Error().Err(err).Msg("Sentry init failed for remote forwarder; continuing without error reporting")
		settings.Sentry = remoteforward.SentryConfig{}
		forwardCfg, err = remoteforward.NewForwardConfig(settings)
		if err != nil {
			panic(err)
		}
	}
	svc.forward = forwardCfg

	return svc
}

// Shutdown flushes monitoring hooks (for example Sentry) for this forwarder.
func (fs *ForwardingService) Shutdown() {
	fs.forward.Shutdown()
}

func (fs *ForwardingService) Apply(request entity.ServiceRequest) (entity.ServiceResponse, error) {
	var serviceResponse entity.ServiceResponse
	var err error

	err = fs.backoff.ExecuteWithBackoff(func() error {
		response, err := fs.forwardRequest(request)
		if err != nil {
			return err
		}

		parsed, err := remoteforward.ReadAndValidateResponse(response, fs.forward)
		if err != nil {
			return err
		}

		serviceResponse = parsed
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

	urlString, err := remoteforward.BuildForwardURL(fs.Host, fs.PathPrefix, request.GetURL(), fs.forward)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, urlString, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := fs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %w", err)
	}

	return resp, nil
}
