package api

import (
	"github.com/QueerGlobal/hub-framework/core/entity"
)

func RegisterTaskType(name string,
	taskConstructor entity.TaskConstructor) {

	entity.RegisterTaskType(name, taskConstructor)
}

func GetTask(taskName string, config map[string]interface{}) (entity.Task, error) {

	return entity.GetTask(taskName, config)
}
