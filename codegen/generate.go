package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func GenerateNewProject(appName string) {
	config := AppConfig{Name: appName}

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
		filepath.Join("aggregates", "example.yaml"):     aggregateYamlTemplate,
		filepath.Join("schemas", "example.schema.json"): schemaJsonTemplate,
		filepath.Join("schemas", "schemas.yaml"):        schemasYamlTemplate,
		"hub.yaml":                                      hubYamlTemplate,
		"main.go":                                       mainGoTemplate,
	}

	// Create and populate files
	for path, content := range files {
		tmpl, err := template.New(filepath.Base(path)).Parse(content)
		if err != nil {
			fmt.Printf("Error parsing template for %s: %v\n", path, err)
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

	fmt.Printf("Application %s generated successfully!\n", appName)
}
