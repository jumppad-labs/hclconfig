package types

import "github.com/hashicorp/hcl2/hcl"

// TypeModule is the resource string for a Module resource
const TypeModule = "module"

// Module allows Shipyard configuration to be imported from external folder or
// GitHub repositories
type Module struct {
	ResourceMetadata `hcl:",remain"`

	Source string `hcl:"source" json:"source"`

	Variables interface{} `hcl:"variables,optional" json:"variables,omitempty"`

	// SubContext is used to store the variables as a context that can be
	// passed to child resources
	SubContext *hcl.EvalContext
}
