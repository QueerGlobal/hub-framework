package requesthandler

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/labstack/echo/v4"
)

type RequestForwarder interface {
	HandleRequest(r *http.Request) (*entity.ServiceResponse, error)
}

type RequestHandler struct {
	hub          RequestForwarder
	echoInstance *echo.Echo
	port         int
}

func NewRequestHandler(
	port int,
	hub RequestForwarder) *RequestHandler {
	return &RequestHandler{
		echoInstance: echo.New(),
		port:         port,
		hub:          hub,
	}
}

func (r *RequestHandler) Start(wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		portStr := ":" + strconv.Itoa(r.port)

		// Remove this line:
		// http.Handle("/", r)

		log.Printf("Starting server on %s...\r\n", portStr)
		if err := http.ListenAndServe(portStr, r); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()
	return nil
}

func (handlerInt *RequestHandler) GetPort() int {
	return handlerInt.port
}

func (handler *RequestHandler) GetHub() RequestForwarder {
	return handler.hub
}

/*
ServeHTTP is the handler for passthrough
requests to our backend services
*/
func (handler *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	response, err := handler.GetHub().HandleRequest(r)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for header, values := range response.ResponseMeta.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}

	w.WriteHeader(response.ResponseMeta.StatusCode)

	if _, err := w.Write(response.Body); err != nil {
		log.Printf("Error writing response body: %v", err)
		return
	}
}
