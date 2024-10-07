package builtin

import (
	"context"
	"log"
	"strings"

	"encoding/json"
	"net/http"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

type RequestLogger struct {
	name     string
	LogLevel string
}

func NewRequestLoggerTask(config map[string]interface{}) (entity.Task, error) {
	rl := &RequestLogger{
		name:     "",
		LogLevel: "INFO",
	}

	if name, ok := config["name"].(string); ok {
		rl.name = name
	}

	if level, ok := config["logLevel"].(string); ok {
		rl.LogLevel = level
	}

	return rl, nil
}

func (rl *RequestLogger) Name() string {
	return rl.name
}

func (rl *RequestLogger) Apply(ctx context.Context, request entity.ServiceRequest) error {
	logMessage := map[string]interface{}{
		"Method": request.GetMethod().String(),
		"URL":    request.GetURL().String(),
		"Headers": func() map[string]string {
			headers := make(map[string]string)
			for k, v := range request.GetHeader() {
				headers[k] = strings.Join(v, ", ")
			}
			return headers
		}(),
	}

	// Deserialize body
	var body interface{}
	err := json.Unmarshal(request.GetBody(), &body)
	if err != nil {
		logMessage["Body"] = string(request.GetBody()) // Fallback to string if not JSON
	} else {
		logMessage["Body"] = body
	}

	// Log response if available
	if response := request.GetResponse(); response != nil {
		var responseBodyUnmarshaled interface{}
		err := json.Unmarshal(response.GetBody(), &responseBodyUnmarshaled)
		if err != nil {
			responseBodyUnmarshaled = string(response.GetBody())
		}

		logMessage["Response"] = map[string]interface{}{
			"StatusCode": response.GetResponseMeta().GetStatusCode(),
			"Status":     http.StatusText(response.GetResponseMeta().GetStatusCode()),
			"Headers":    response.GetResponseMeta().GetHeader(),
			"Body":       responseBodyUnmarshaled,
		}
	}

	logJSON, _ := json.MarshalIndent(logMessage, "", "  ")
	log.Printf("[%s] Request Log:\n%s", rl.LogLevel, string(logJSON))

	return nil
}
