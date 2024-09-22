package model

import "gopkg.in/yaml.v2"

type AggregateSpec struct {
	APIVersion string    `yaml:"apiVersion"`
	SpecType   string    `yaml:"specType"`
	Namespace  string    `yaml:"namespace"`
	Spec       Aggregate `yaml:"spec"`
}

type Aggregate struct {
	Name          string    `yaml:"name"`
	APIName       string    `yaml:"apiName"`
	IsPublic      bool      `yaml:"isPublic"`
	SchemaName    string    `yaml:"schemaName"`
	SchemaVersion string    `yaml:"schemaVersion"`
	Refs          []string  `yaml:"refs"`
	Handlers      []Handler `yaml:"handlers"`
}

type Target struct {
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config,omitempty"`
}

type Handler struct {
	Methods  []string `yaml:"methods"`
	Inbound  []Task   `yaml:"inbound"`
	Outbound []Task   `yaml:"outbound"`
	Target   Target   `yaml:"target"`
}

type Task struct {
	Name          string                 `yaml:"name"`
	Type          string                 `yaml:"type"`
	Description   string                 `yaml:"description,omitempty"`
	Precedence    int                    `yaml:"precedence,omitempty"`
	ExecutionType string                 `yaml:"executionType,omitempty"`
	MustFinish    bool                   `yaml:"mustFinish,omitempty"`
	OnError       string                 `yaml:"onError,omitempty"`
	Enabled       bool                   `yaml:"enabled,omitempty"`
	Config        map[string]interface{} `yaml:"config,omitempty"`
}

func UnmarshalAggregate(specYaml []byte) (*AggregateSpec, error) {
	var aggregateSpec AggregateSpec
	if err := yaml.Unmarshal(specYaml, &aggregateSpec); err != nil {
		return nil, err
	}

	return &aggregateSpec, nil
}
