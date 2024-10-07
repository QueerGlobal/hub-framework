package error

import (
	"errors"
)

var ErrEmptyInput error = errors.New("empty input")

var ErrUnsupportedHTTPMethod = errors.New("unsupported HTTP method")

var ErrEmptyResponse error = errors.New("unexpected empty response from target")

var ErrWorkflowTaskNotRegistered = errors.New("workflow task not registered")

var ErrTargetNotRegistered = errors.New("workflow target not registered")

var ErrTargetNotConfigured = errors.New("no target configured for this service")

var ErrServiceNotFound = errors.New("no such service found")

var ErrSerializationVersionMismatch = errors.New(
	"serialization version mismatch")

var ErrSerializationTypeMismatch = errors.New(
	"serialization version mismatch")

var ErrTargetTypeNotSupported = errors.New("target type not supported")
