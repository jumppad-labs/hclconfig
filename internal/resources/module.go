package resources

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/xcl/types"
)

// TypeModule is the resource string for a Module resource
const TypeModule = "module"

// Module allows Shipyard configuration to be imported from external folder or
// GitHub repositories
type Module struct {
	types.ResourceBase `hcl:",remain"`

	Source  string `hcl:"source" json:"source"`
	Version string `hcl:"version,optional" json:"version,omitempty"`

	// Variables is captured as a raw expression rather than decoded to a
	// concrete Go type: gocty's implied-type decode can't represent a
	// heterogeneous object (e.g. {cpu = 1, enabled = true}) as a single cty
	// type, since it forces every map value to share one element type. The
	// expression is evaluated manually into a cty.Value during the walk
	// callback, once a full context is available.
	Variables hcl.Expression `hcl:"variables,optional" json:"-"`

	// SubContext is used to store the variables as a context that can be
	// passed to child resources
	SubContext *hcl.EvalContext
}
