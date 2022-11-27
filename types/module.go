package types

import "github.com/hashicorp/hcl2/hcl"

// TypeModule is the resource string for a Module resource
const TypeModule ResourceType = "module"

// Module allows Shipyard configuration to be imported from external folder or
// GitHub repositories
type Module struct {
	ResourceInfo `hcl:",remain" mapstructure:",squash"`

	Depends []string `hcl:"depends_on,optional" json:"depends,omitempty"`

	Source string `hcl:"source" json:"source"`

	Variables interface{} `hcl:"variables,optional" json:"variables,omitempty"`

	// SubContext is used to store the variables as a context that can be
	// passed to child resources
	SubContext *hcl.EvalContext
}

// New creates a new Module config resource, implements Resource New method
func (t *Module) New(name string) Resource {
	return &Module{ResourceInfo: ResourceInfo{Name: name, Type: TypeModule, Status: PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (t *Module) Info() *ResourceInfo {
	return &t.ResourceInfo
}

func (t *Module) Parse(file string) error {
	return nil
}

func (t *Module) Process() error {
	return nil
}
