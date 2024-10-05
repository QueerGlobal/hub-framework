package main_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/api"
	"github.com/QueerGlobal/hub-framework/example/recipe-app/golang/tasks"
)

func TestApplication(t *testing.T) {
	// Create a new application instance
	app := api.NewApplication("testApp", api.WithLogLevel(api.InfoLevel))
	if app == nil {
		t.Fatal("Error creating application")
	}

	// Register the example task
	exampleTaskConstructor := api.TaskConstructorFunc(func(config map[string]any) (api.Task, error) {
		return tasks.NewExampleTaskGolang(config), nil
	})
	api.RegisterTaskType("exampleTaskGolang", exampleTaskConstructor)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to testApp!")
	}))
	defer server.Close()

	// Start the application in a goroutine
	go func() {
		err := app.Start()
		if err != nil {
			t.Errorf("Error starting application: %v", err)
		}
	}()

	// Give the application some time to start
	time.Sleep(100 * time.Millisecond)

	// Send an HTTP request to the test server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK; got %v", resp.Status)
	}

	// Stop the application
	app.Stop()
}
