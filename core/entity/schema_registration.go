package entity

import (
	"fmt"
	"sync"
)

type Schema struct {
	Name    string
	Version string
	Data    []byte
}

var (
	schemaRegistry     map[string]Schema
	schemaRegistryOnce sync.Once
)

func SchemaRegistry() map[string]Schema {
	schemaRegistryOnce.Do(func() {
		schemaRegistry = make(map[string]Schema)
	})
	return schemaRegistry
}

func RegisterSchema(name string, version string, data []byte) {
	key := fmt.Sprintf("%s:%s", name, version)
	SchemaRegistry()[key] = Schema{
		Name:    name,
		Version: version,
		Data:    data,
	}
}

func GetSchema(name string, version string) (Schema, bool) {
	key := fmt.Sprintf("%s:%s", name, version)
	schema, ok := SchemaRegistry()[key]
	return schema, ok
}
