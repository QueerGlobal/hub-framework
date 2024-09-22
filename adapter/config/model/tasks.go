package model

import "gopkg.in/yaml.v2"

type TasksSpec struct {
	APIVersion string `yaml:"apiVersion"`
	SpecType   string `yaml:"specType"`
	Namespace  string `yaml:"namespace"`
	Spec       []Task `yaml:"spec"`
}

func UnmarshalTasks(specYaml []byte) (*TasksSpec, error) {
	var tasksSpec TasksSpec
	if err := yaml.Unmarshal(specYaml, &tasksSpec); err != nil {
		return nil, err
	}

	return &tasksSpec, nil
}
