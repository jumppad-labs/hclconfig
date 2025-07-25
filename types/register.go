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

type RegisteredTypes map[string]any

// CreateResource creates a new instance of a resource from one of the registered types.
func (r RegisteredTypes) CreateResource(resourceType, resourceName string) (any, error) {
	// check that the type exists
	if t, ok := r[resourceType]; ok {
		ptr := reflect.New(reflect.TypeOf(t).Elem())

		res := ptr.Interface()

		meta, _ := GetMeta(res)
		meta.Name = resourceName
		meta.Type = resourceType
		meta.Properties = make(map[string]any)

		return res, nil
	}

	return nil, fmt.Errorf("unable to create resource: %s", NewTypeNotRegisteredError(resourceType))
}
