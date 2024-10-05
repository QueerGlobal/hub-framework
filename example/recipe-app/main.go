package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/QueerGlobal/hub-framework/api"
	"github.com/QueerGlobal/hub-framework/example/recipe-app/golang/tasks"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to {{.ApplicationName}}!")
	})

	// Example of using the API client
	app := api.NewApplication("{{.ApplicationName}}", api.WithLogLevel(api.InfoLevel))
	if app == nil {
		log.Printf("Error creating application")
		return
	}

	exampleTaskConstructor := api.TaskConstructorFunc(func(config map[string]any) (api.Task, error) {
		return tasks.NewExampleTaskGolang(config), nil
	})

	api.RegisterTaskType("exampleTaskGolang", exampleTaskConstructor)

	log.Printf("Starting {{.ApplicationName}} server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
