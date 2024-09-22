package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/atombender/go-jsonschema/pkg/generator"
)

func main() {
	input := flag.String("input", "", "Path to a JSON schema file or directory containing JSON schema files")
	output := flag.String("output", "", "Directory to place the generated Go files")
	flag.Parse()

	if *input == "" || *output == "" {
		log.Fatal("Both input and output paths are required")
	}

	var files []string
	err := filepath.Walk(*input, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error reading input: %v", err)
	}

	for _, file := range files {
		opts := generator.Config{
			DefaultPackageName: "main", // Change to your desired package name
			DefaultOutputName:  *output,
		}
		gen, err := generator.New(opts)
		if err != nil {
			log.Fatalf("Error bulding generator with with options %+v: %v", opts, err)
		}
		if err := gen.DoFile(file); err != nil {
			log.Fatalf("Error generating Go code for %s: %v", file, err)
		}
	}

	fmt.Println("Code generation complete.")
}
