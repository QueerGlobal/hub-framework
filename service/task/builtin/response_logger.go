package builtin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

type ResponseLogger struct {
	name     string
	LogLevel string
}

func NewResponseLoggerTask(config map[string]interface{}) (entity.Task, error) {
	rl := &ResponseLogger{
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

func (rl *ResponseLogger) Name() string {
	return rl.name
}

func (rl *ResponseLogger) Apply(ctx context.Context, request entity.ServiceRequest) error {
	response := request.GetResponse()
	if response == nil {
		log.Printf("[%s] No response available to log", rl.LogLevel)
		return nil
	}

	logMessage := map[string]interface{}{
		"StatusCode": response.GetResponseMeta().GetStatusCode(),
		"Status":     http.StatusText(response.GetResponseMeta().GetStatusCode()),
		"Headers": func() map[string]string {
			headers := make(map[string]string)
			for k, v := range response.GetResponseMeta().GetHeader() {
				headers[k] = strings.Join(v, ", ")
			}
			return headers
		}(),
	}

	// Deserialize body
	var body interface{}
	err := json.Unmarshal(response.GetBody(), &body)
	if err != nil {
		logMessage["Body"] = string(response.GetBody()) // Fallback to string if not JSON
	} else {
		logMessage["Body"] = body
	}

	logJSON, _ := json.MarshalIndent(logMessage, "", "  ")
	log.Printf("[%s] Response Log:\n%s", rl.LogLevel, string(logJSON))

	return nil
}
