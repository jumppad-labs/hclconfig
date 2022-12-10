package types

import (
	"fmt"
	"reflect"
)

var TypeNotRegisteredError = fmt.Errorf("type not registered")

type RegisteredTypes map[string]Resource

func DefaultTypes() RegisteredTypes {
	return RegisteredTypes{
		"variable": &Variable{},
		"output":   &Output{},
		"module":   &Module{},
	}
}

func (r RegisteredTypes) CreateResource(resourceType, resourceName string) (Resource, error) {
	// check that the type exists
	if t, ok := r[resourceType]; ok {
		ptr := reflect.New(reflect.TypeOf(t).Elem())

		res := ptr.Interface().(Resource)
		res.Metadata().Name = resourceName
		res.Metadata().Type = resourceType
		res.Metadata().Status = PendingProcess

		return res, nil
	}

	return nil, TypeNotRegisteredError
}
