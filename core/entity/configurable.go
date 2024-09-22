package entity

type Configurable interface {
	Configure(map[string]interface{})
	GetConfig() interface{}
}
