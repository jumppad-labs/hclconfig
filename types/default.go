package types

import (
	"fmt"
	"reflect"
)

var TypeNotRegisteredError = fmt.Errorf("type not registered")

type RegisteredTypes map[string]Resource

// DefaultTypes is a collection of the default config types
func DefaultTypes() RegisteredTypes {
	return RegisteredTypes{
		"variable": &Variable{},
		"output":   &Output{},
		"module":   &Module{},
	}
}

// CreateResource creates a new instance of a resource from one of the registered types.
func (r RegisteredTypes) CreateResource(resourceType, resourceName string) (Resource, error) {
	// check that the type exists
	if t, ok := r[resourceType]; ok {
		ptr := reflect.New(reflect.TypeOf(t).Elem())

		res := ptr.Interface().(Resource)
		res.Metadata().Name = resourceName
		res.Metadata().Type = resourceType
		res.Metadata().Properties = map[string]interface{}{}

		return res, nil
	}

	return nil, TypeNotRegisteredError
}
