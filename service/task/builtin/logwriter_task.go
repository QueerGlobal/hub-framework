package builtin

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

type LogWriter struct {
	name     string
	LogLevel string
	Fields   []LogField
}

type LogField struct {
	Name  string
	Value string
}

func NewLogWriterTask(config map[string]interface{}) (entity.Task, error) {
	lw := &LogWriter{
		name:     "",
		LogLevel: "INFO",
	}

	if name, ok := config["name"].(string); ok {
		lw.name = name
	}

	if level, ok := config["logLevel"].(string); ok {
		lw.LogLevel = level
	}

	if fields, ok := config["fields"].([]interface{}); ok {
		for _, field := range fields {
			if fieldMap, ok := field.(map[string]interface{}); ok {
				lw.Fields = append(lw.Fields, LogField{
					Name:  fieldMap["name"].(string),
					Value: fieldMap["value"].(string),
				})
			}
		}
	}

	return lw, nil
}

func (lw *LogWriter) Name() string {
	return lw.name
}

func (lw *LogWriter) Apply(ctx context.Context, request entity.ServiceRequest) error {
	logMessage := make(map[string]string)

	for _, field := range lw.Fields {
		value, err := lw.extractCommonFields(field.Value, request)
		if err == nil {
			logMessage[field.Name] = value
			continue
		}

		// If not a common field, fall back to reflection-based extraction
		value, err = lw.extractValue(field.Value, request)
		if err != nil {
			return fmt.Errorf("failed to extract value for field %s: %v", field.Name, err)
		}
		logMessage[field.Name] = value
	}

	// Log the message
	log.Printf("[%s] %v", lw.LogLevel, logMessage)

	return nil
}

func (lw *LogWriter) extractCommonFields(pattern string, request entity.ServiceRequest) (string, error) {
	if !strings.HasPrefix(pattern, "{{") || !strings.HasSuffix(pattern, "}}") {
		return pattern, nil
	}

	path := strings.Trim(pattern, "{}")
	parts := strings.Split(path, ".")

	switch {
	case len(parts) == 2 && parts[0] == "Request" && parts[1] == "Body":
		return string(request.GetBody()), nil
	case len(parts) == 2 && parts[0] == "Response" && parts[1] == "StatusCode":
		return fmt.Sprintf("%d", request.GetResponse().GetResponseMeta().GetStatusCode()), nil
	case len(parts) == 2 && parts[0] == "Request" && parts[1] == "Method":
		return request.GetMethod().String(), nil
	case len(parts) == 3 && parts[0] == "Request" && parts[1] == "URL" && parts[2] == "Path":
		return request.GetURL().Path, nil
	// Add more common fields as needed
	default:
		return "", fmt.Errorf("not a common field: %s", pattern)
	}
}

func (lw *LogWriter) extractValue(pattern string, request entity.ServiceRequest) (string, error) {
	if !strings.HasPrefix(pattern, "{{") || !strings.HasSuffix(pattern, "}}") {
		return pattern, nil
	}

	path := strings.Trim(pattern, "{}")
	parts := strings.Split(path, ".")

	var value interface{} = request
	for _, part := range parts {
		v := reflect.ValueOf(value)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		if v.Kind() != reflect.Struct {
			return "", fmt.Errorf("invalid path: %s is not a struct", part)
		}

		f := v.FieldByName(part)
		if !f.IsValid() {
			return "", fmt.Errorf("field not found: %s", part)
		}

		value = f.Interface()
	}

	return fmt.Sprintf("%v", value), nil
}
