package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func GenerateNewProject(config map[string]interface{}) {
	// Create directories
	dirs := []string{"aggregates", "schemas"}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			fmt.Printf("Error creating directory %s: %v\n", dir, err)
			return
		}
	}

	// Define file templates
	files := map[string]string{
		filepath.Join("aggregates", "example.yaml"):     "example.yaml.tmpl",
		filepath.Join("schemas", "example.schema.json"): "example.schema.json.tmpl",
		filepath.Join("schemas", "schemas.yaml"):        "schemas.yaml.tmpl",
		"hub.yaml":                                      "hub.yaml.tmpl",
		"main.go":                                       "main.go.tmpl",
	}

	// Create and populate files
	for path, templateFile := range files {
		tmplPath := filepath.Join("codegen", templateFile)
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			fmt.Printf("Error parsing template file %s: %v\n", tmplPath, err)
			return
		}

		file, err := os.Create(path)
		if err != nil {
			fmt.Printf("Error creating file %s: %v\n", path, err)
			return
		}
		defer file.Close()

		err = tmpl.Execute(file, config)
		if err != nil {
			fmt.Printf("Error executing template for %s: %v\n", path, err)
			return
		}
	}

	fmt.Printf("Application %s generated successfully!\n", config["ApplicationName"])
}
