package model

type WorkflowState struct {
	RequestWorkflowChainState  WorkflowChainState
	ResponseWorkflowChainState WorkflowChainState
	TargetState                TargetState
}

type WorkflowChainState struct {
	Steps []WorkflowChainStepState
}

type WorkflowChainStepState struct {
	TaskName   string
	Precedence int
	Config     any
}
