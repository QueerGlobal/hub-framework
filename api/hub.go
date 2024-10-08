package api

import (
	"net/http"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/rs/zerolog"
)

type Hub interface {
	HandleRequest(r *http.Request) (entity.ServiceResponse, error)
	SetLogger(l *zerolog.Logger)
	GetLogger() *zerolog.Logger
	GetServices() map[string]*entity.Service
}
