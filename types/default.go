package types

import (
	"fmt"
	"reflect"
	"strings"
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

type TypeDefinition struct {
	HasSubType bool
}

// TopLevelTypes are the allowed top level types for the configuration
type TopLevelTypesList map[string]TypeDefinition

// return the keys as a comma separated list
func (tlt *TopLevelTypesList) Keys() string {
	keys := ""
	for k, _ := range *tlt {
		keys += fmt.Sprintf("%s, ", k)
	}

	return strings.TrimPrefix(keys, ", ")
}

// Contains checks if the given string is in the types list
func (tlt *TopLevelTypesList) Contains(t string) bool {

	for k, _ := range *tlt {
		if t == k {
			return true
		}
	}

	return false
}

var TopLevelTypes = TopLevelTypesList{
	"resource": TypeDefinition{HasSubType: true},
	"output":   TypeDefinition{HasSubType: false},
	"module":   TypeDefinition{HasSubType: false},
	"variable": TypeDefinition{HasSubType: false},
	"scenario": TypeDefinition{HasSubType: false},
	"test":     TypeDefinition{HasSubType: false},
}

// DefaultTypes is a collection of the default config types
func DefaultTypes() RegisteredTypes {
	return RegisteredTypes{
		"variable": &Variable{},
		"output":   &Output{},
		"local":    &Local{},
		"module":   &Module{},
		"root":     &Root{},
		"scenario": &Scenario{},
		"test":     &Test{},
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
