package resources

import (
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)

const TypeVariable = "variable"

// Variable defines a variable which can be referenced by resources
type Variable struct {
	types.ResourceBase `hcl:",remain"`
	Default            cty.Value `hcl:"default" json:"default"`                            // default value for a variable
	Description        string    `hcl:"description,optional" json:"description,omitempty"` // description of the variable
}
