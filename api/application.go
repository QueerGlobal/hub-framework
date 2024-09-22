package api

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/QueerGlobal/qg-hub/adapter/config/yaml"
	"github.com/QueerGlobal/qg-hub/adapter/handler/requesthandler"
	"github.com/QueerGlobal/qg-hub/core/entity"
	"github.com/rs/zerolog"
)

type Application struct {
	ApplicationName string
	ApplicationHome string
	PrivatePort     int
	PublicPort      int
	CustomTaskTypes []entity.TaskConstructor
	CustomTargets   []entity.TargetConstructor
	LogLevel        LogLevel
	Hub             Hub
	PublicHandler   *requesthandler.RequestHandler
	PrivateHandler  *requesthandler.RequestHandler
	Logger          *zerolog.Logger
}

type Option func(*Application)

func WithCustomTargets(targets ...entity.TargetConstructor) Option {
	return func(a *Application) {
		a.CustomTargets = targets
	}
}

func WithCustomTasks(targets ...entity.TaskConstructor) Option {
	return func(a *Application) {
		a.CustomTaskTypes = targets
	}
}

func WithLogLevel(level LogLevel) Option {
	return func(a *Application) {
		a.LogLevel = level
	}
}

func WithPublicPort(port int) Option {
	return func(a *Application) {
		a.PublicPort = port
	}
}

func WithPrivatePort(port int) Option {
	return func(a *Application) {
		a.PrivatePort = port
	}
}

func WithApplicationHome(home string) Option {
	return func(app *Application) {
		app.ApplicationHome = home
	}
}

func NewApplication(applicationName string, opts ...Option) *Application {
	// Default settings
	s := Application{
		ApplicationHome: "./",
		ApplicationName: applicationName,
		PrivatePort:     3532,
		PublicPort:      3531,
		CustomTaskTypes: make([]entity.TaskConstructor, 0),
		CustomTargets:   make([]entity.TargetConstructor, 0),
		LogLevel:        InfoLevel,
	}
	// Apply functional options
	for _, opt := range opts {
		opt(&s)
	}

	return &s
}

func (a *Application) createLogger() *zerolog.Logger {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	return &logger
}

func (a *Application) createHub(applicationName string, logger *zerolog.Logger) (Hub, error) {
	hub, err := entity.NewHub(logger, applicationName)
	if err != nil {
		logger.Err(err).Msgf("error initializing hub")
		return nil, err
	}

	return hub, err
}

func (a *Application) startHub() error {
	hub := a.Hub.(*entity.Hub)

	publicHandler := requesthandler.NewRequestHandler(a.PublicPort, hub)
	//privateHandler := requesthandler.NewRequestHandler(a.PrivatePort, hub)

	handlerWG := sync.WaitGroup{}
	if err := publicHandler.Start(&handlerWG); err != nil {
		log.Print("error initializing public handler ")
		log.Println(err)
		return err
	}

	/*
		TODO:  Registering both the public and private handlers creates
		a conlfict where the private handler is not able to start
		as the public handler is already running.

			if err := privateHandler.Start(&handlerWG); err != nil {
				log.Print("error initializing private handler ")
				log.Println(err)
				return nil, nil, err
			}
	*/

	a.PrivateHandler = nil // temporary until we have a private handler
	a.PublicHandler = publicHandler

	return nil
}

func (a *Application) Start() error {
	logger := a.createLogger()

	hub, err := a.createHub(a.ApplicationName, logger)
	if err != nil {
		err = fmt.Errorf("failed to start hub service: %w", err)
		log.Println(err)
		return err
	}

	a.Hub = hub

	configurer := yaml.NewConfigurer(a.ApplicationHome)

	if hub == nil {
		return fmt.Errorf("failed to create hub")
	}

	err = configurer.ConfigureHub(hub.(*entity.Hub))
	if err != nil {
		err = fmt.Errorf("failed to configure hub: %w", err)
		log.Println(err)
		return err
	}

	if err := a.startHub(); err != nil {
		err = fmt.Errorf("failed to start hub service: %w", err)
		log.Println(err)
		return err
	}

	return nil
}

func (a *Application) Stop() error {
	return nil
}
