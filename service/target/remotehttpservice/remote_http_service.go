package remotehttpservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func NewForwardingService(cfg ForwardingServiceConfig) (ForwardingService, error) {
	svc := ForwardingService{}

	svc.Host = cfg.Host
	svc.PathPrefix = remoteforward.NormalizePathPrefix(cfg.PathPrefix)

	backoffConfig := cfg.BackoffConfig
	backoffConfig.TargetURL = cfg.Host
	if backoffConfig.Logger == nil {
		backoffConfig.Logger = cfg.Logger
	}
	backoff := util.NewBackoff(backoffConfig)
	svc.backoff = backoff
	svc.httpClient = remoteforward.NewHTTPClient(cfg.ClientTimeout)
	settings := remoteforward.ForwardSettings{
		MaxResponseBodyBytes: cfg.MaxResponseBodyBytes,
		Logger:               cfg.Logger,
		Sentry:               cfg.Sentry,
		UpstreamHost:         cfg.Host,
	}
	forwardCfg, err := remoteforward.NewForwardConfigResilient(settings)
	if err != nil {
		if cfg.Logger != nil {
			cfg.Logger.Error().Err(err).Msg("remote forward config failed")
		} else {
			log := remoteforward.DefaultLogger()
			log.Error().Err(err).Msg("remote forward config failed")
		}
		return ForwardingService{}, err
	}
	svc.forward = forwardCfg

	return svc, nil
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

		var payload map[string]interface{}
		if err := json.Unmarshal(parsed.GetBody(), &payload); err != nil {
			return fmt.Errorf("error unmarshaling response: %w: %w", util.ErrUnrecoverable, err)
		}

		serviceResponse = &forwardedJSONResponse{
			meta: parsed.GetResponseMeta(),
			data: payload,
		}

		return nil
	})
	if err != nil {
		if remoteforward.ShouldReportExhaustedHTTPFailure(err) {
			fs.forward.ReportHTTPFailure(err)
		}
		return nil, util.RedactURLError(err)
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
		return nil, fmt.Errorf("failed to perform HTTP request: %w", util.RedactURLError(err))
	}

	return resp, nil
}
