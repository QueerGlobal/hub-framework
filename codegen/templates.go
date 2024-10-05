package codegen

type AppConfig struct {
	Name string
}

const aggregateYamlTemplate = `
name: {{.AggregateName}}
description: Example aggregate {{.ApplicationName}}
fields:
  - name: id
    type: string
  - name: name
    type: string
  - name: created_at
    type: timestamp
`

const schemaJsonTemplate = `
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "created_at": {
      "type": "string",
      "format": "date-time"
    }
  },
  "required": ["id", "name", "created_at"]
}
`

const schemasYamlTemplate = `
schemas:
  - name: {{.SchemaName}}
    file: example.schema.json
`

const hubYamlTemplate = `
name: {{.ApplicationName}}
version: 1.0.0
description: Hub configuration for {{.ApplicationName}}
aggregates:
  - aggregates/example.yaml
schemas:
  - schemas/schemas.yaml
`

const mainGoTemplate = `
package main

import (
    "fmt"
    "log"
    "net/http"
    "github.com/QueerGlobal/hub-framework/api"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to {{.ApplicationName}}!")
	})

	// Example of using the API client
	client := api.NewApplication("{{.ApplicationName}}", api.WithLogLevel(api.LogLevelDebug))
	result, err := client.GetSomething()
	if err != nil {
		log.Printf("Error calling API: %v", err)
	} else {
		log.Printf("API result: %s", result)
	}

	log.Printf("Starting {{.ApplicationName}} server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`
