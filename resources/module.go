package resources

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/hclconfig/types"
)

// TypeModule is the resource string for a Module resource
const TypeModule = "module"

// Module allows Shipyard configuration to be imported from external folder or
// GitHub repositories
type Module struct {
	types.ResourceBase `hcl:",remain"`

	Source  string `hcl:"source" json:"source"`
	Version string `hcl:"version,optional" json:"version,omitempty"`

	Variables interface{} `hcl:"variables,optional" json:"variables,omitempty"`

	// SubContext is used to store the variables as a context that can be
	// passed to child resources
	SubContext *hcl.EvalContext
}
