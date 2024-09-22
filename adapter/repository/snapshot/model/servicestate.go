package model

type ServiceState struct {
	Name      string
	Path      string
	IsPublic  bool
	Targets   []TargetState
	Methods   []string
	Workflows map[string]*WorkflowState
}

//MethodConfigs map[string]WorkflowConfig
