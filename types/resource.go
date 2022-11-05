package types

import "fmt"

// ResourceType is the type of the resource
type ResourceType string

// Status defines the current state of a resource
type Status string

// Applied means the resource has been successfully created
const Applied Status = "applied"

// PendingCreation means the resource has not yet been created
// it will be created on the next run
const PendingCreation Status = "pending_creation"

// PendingModification means the resource has been created but
// if the action is Apply then the resource will be re-created with the next run
// if the action is Delete then the resource will be removed with the next run
const PendingModification Status = "pending_modification"

// PendingUpdate means the resource has been requested to be updated
// if the action is Apply then the resource will be ignored with the next run
// if the action is Delete then the resource will be removed with the next run
const PendingUpdate Status = "pending_update"

// Failed means the resource failed during creation
// if the action is Apply the resource will be re-created at the next run
const Failed Status = "failed"

// Destroyed means the resource has been destroyed
const Destroyed Status = "destroyed"

// Disabled means the resource will be ignored by the engine and no resources
// will be created or destroyed
const Disabled Status = "disabled"

// Resource defines a base interface that all resources must implement
type Resource interface {
	// defines a fake constructor that every type must implement
	New(name string) Resource
	// Info returns the base info for a resource type
	Info() *ResourceInfo
	// Parse is called by the parser when the resource is deserialized from a file
	Parse(configFile string)
}

// ResourceInfo is the embedded type for any config resources
// it deinfes common meta data that all resources share
type ResourceInfo struct {
	// Name is the name of the resource
	Name string `json:"name"`
	// Type is the type of resource, this is the text representation of the golang type
	Type ResourceType `json:"type"`
	// Status is the current status of the resource, this is always PendingCreation initially
	Status Status `json:"status,omitempty"`
	// Module is the name of the module if a resource has been loaded from a module
	Module string `json:"module,omitempty"`

	// DependsOn is a list of objects which must exist before this resource can be applied
	DependsOn []string `json:"depends_on,omitempty" mapstructure:"depends_on"`

	ResouceLinks map[string]string `json:"resource_links,omitempty" mapstructure:"resource_links"`

	// Enabled determines if a resource is enabled and should be processed
	Disabled bool `hcl:"disabled,optional" json:"disabled,omitempty"`
}

var TypeNotRegisteredError = fmt.Errorf("type not registered")

type RegisteredTypes map[string]Resource

func DefaultTypes() RegisteredTypes {
	return RegisteredTypes{
		"variable": (&Variable{}).New(""),
		"output":   (&Output{}).New(""),
		"module":   (&Module{}).New(""),
	}
}

func (r RegisteredTypes) CreateResource(resourceType, resourceName string) (Resource, error) {
	// check that the type exists
	if t, ok := r[resourceType]; ok {
		return t.New(resourceName), nil
	}

	return nil, TypeNotRegisteredError
}
