package resources

import "github.com/jumppad-labs/hclconfig/types"

const TypeVariable = "variable"

// Output defines an output variable which can be set by a module
type Variable struct {
	types.ResourceBase `hcl:",remain"`

	Type        string `hcl:"type" json:"type"`                                  // type of the variable e.g. string, number, bool, object, list
	Default     any    `hcl:"default,optional" json:"default"`                   // default value for a variable
	Description string `hcl:"description,optional" json:"description,omitempty"` // description of the variable
}
