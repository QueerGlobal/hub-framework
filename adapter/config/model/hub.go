package model

import "gopkg.in/yaml.v2"

type HubSpec struct {
	APIVersion string `yaml:"apiVersion"`
	SpecType   string `yaml:"specType"`
	Spec       Hub    `yaml:"spec"`
}

type Hub struct {
	ApplicationName    string `yaml:"applicationName"`
	ApplicationVersion string `yaml:"applicationVersion"`
	PublicPort         int    `yaml:"publicPort"`
	PrivatePort        int    `yaml:"privatePort"`
	APIs               []API  `yaml:"apis"`
}

type API struct {
	Name       string            `yaml:"name"`
	Aggregates map[string]string `yaml:"aggregates"`
}

func UnmarshalHub(specYaml []byte) (*HubSpec, error) {
	var hubSpec HubSpec
	if err := yaml.Unmarshal(specYaml, &hubSpec); err != nil {
		return nil, err
	}

	return &hubSpec, nil
}
