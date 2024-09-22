package model

type HubState struct {
	Version       uint64
	ServiceStates []*ServiceState
}
