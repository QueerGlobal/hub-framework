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
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ForwardingService struct {
	Host       string
	PathPrefix string
	name       string
	backoff    *util.Backoff
	httpClient *http.Client
	forward    remoteforward.ForwardConfig
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

	var maxResponseBodyBytes int64
	if maxBody, ok := config["maxResponseBodyBytes"].(float64); ok {
		maxResponseBodyBytes = int64(maxBody)
	}

	sentrySettings := remoteforward.SentryConfig{}
	if sentryCfg, ok := config["sentry"].(map[string]interface{}); ok {
		if dsn, ok := sentryCfg["dsn"].(string); ok {
			sentrySettings.DSN = dsn
		}
		if env, ok := sentryCfg["environment"].(string); ok {
			sentrySettings.Environment = env
		}
		if release, ok := sentryCfg["release"].(string); ok {
			sentrySettings.Release = release
		}
		if sampleRate, ok := sentryCfg["sampleRate"].(float64); ok {
			sentrySettings.SampleRate = sampleRate
		}
	}
	var logger *zerolog.Logger
	if logCfg, ok := config["logger"].(*zerolog.Logger); ok {
		logger = logCfg
	}

	backoffConfig.TargetURL = svc.Host
	if backoffConfig.Logger == nil {
		backoffConfig.Logger = logger
	}
	backoff := util.NewBackoff(backoffConfig)
	svc.backoff = backoff
	svc.httpClient = remoteforward.NewHTTPClient(clientTimeout)
	forwardCfg, err := remoteforward.NewForwardConfigResilient(remoteforward.ForwardSettings{
		MaxResponseBodyBytes: maxResponseBodyBytes,
		Logger:               logger,
		Sentry:               sentrySettings,
		UpstreamHost:         svc.Host,
	})
	if err != nil {
		return nil, err
	}
	svc.forward = forwardCfg

	return &svc, nil
}

// NewForwardingServiceWithSettings builds a remote forwarder task from explicit forward settings (for tests).
func NewForwardingServiceWithSettings(
	host string,
	pathPrefix string,
	settings remoteforward.ForwardSettings,
	backoffConfig util.BackoffConfig,
	clientTimeout time.Duration,
) (entity.Task, error) {
	if len(pathPrefix) > 0 {
		if !strings.HasPrefix(pathPrefix, "/") {
			pathPrefix = "/" + pathPrefix
		}
		if !strings.HasSuffix(pathPrefix, "/") {
			pathPrefix = pathPrefix + "/"
		}
	}
	settings.UpstreamHost = host
	forwardCfg, err := remoteforward.NewForwardConfig(settings)
	if err != nil {
		return nil, err
	}
	backoffConfig.TargetURL = host
	backoff := util.NewBackoff(backoffConfig)
	return &ForwardingService{
		Host:       host,
		PathPrefix: pathPrefix,
		backoff:    backoff,
		httpClient: remoteforward.NewHTTPClient(clientTimeout),
		forward:    forwardCfg,
	}, nil
}

// Shutdown flushes monitoring hooks (for example Sentry) for this forwarder.
func (fs *ForwardingService) Shutdown() {
	fs.forward.Shutdown()
}

func (fs *ForwardingService) Apply(ctx context.Context, request entity.ServiceRequest) error {
	err := fs.backoff.ExecuteWithBackoff(func() error {
		response, err := fs.forwardRequest(ctx, request)
		if err != nil {
			return err
		}

		responseObj, err := remoteforward.ReadAndValidateResponse(response, fs.forward)
		if err != nil {
			return err
		}

		request.SetResponse(responseObj)

		return nil
	})
	if err != nil {
		if remoteforward.ShouldReportExhaustedHTTPFailure(err) {
			fs.forward.ReportHTTPFailure(err)
		}
		return util.RedactURLError(err)
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

	urlString, err := remoteforward.BuildForwardURL(fs.Host, fs.PathPrefix, request.GetURL(), fs.forward)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlString, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Inject the trace context into the request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := fs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %w", util.RedactURLError(err))
	}

	return resp, nil
}
