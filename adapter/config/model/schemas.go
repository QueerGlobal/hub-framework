package model

import "gopkg.in/yaml.v2"

type SchemasSpec struct {
	APIVersion string  `yaml:"apiVersion"`
	SpecType   string  `yaml:"specType"`
	Spec       Schemas `yaml:"spec"`
}

type Schemas struct {
	Compatibility string   `yaml:"compatibility"`
	Schemas       []Schema `yaml:"schemas"`
}

type Schema struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	File     string `yaml:"file,omitempty"`
	FileName string `yaml:"filename,omitempty"`
}

func UnmarshalSchemas(specYaml []byte) (*SchemasSpec, error) {
	var schemaSpec SchemasSpec
	if err := yaml.Unmarshal(specYaml, &schemaSpec); err != nil {
		return nil, err
	}

	return &schemaSpec, nil
}
