package types

import (
	"fmt"
	"reflect"
)

type ErrTypeNotRegistered struct {
	Type string
}

func (e *ErrTypeNotRegistered) Error() string {
	return fmt.Sprintf("type %s, not registered", e.Type)
}

func NewTypeNotRegisteredError(t string) *ErrTypeNotRegistered {
	return &ErrTypeNotRegistered{Type: t}
}

type RegisteredTypes map[string]Resource

// DefaultTypes is a collection of the default config types
func DefaultTypes() RegisteredTypes {
	return RegisteredTypes{
		"variable": &Variable{},
		"output":   &Output{},
		"local":    &Local{},
		"module":   &Module{},
		"root":     &Root{},
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

	return nil, fmt.Errorf("unable to create resource: %s", NewTypeNotRegisteredError(resourceType))
}
