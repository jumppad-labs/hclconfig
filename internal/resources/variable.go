package resources

import (
	"github.com/jumppad-labs/xcl/types"
	"github.com/jumppad-labs/xcl/internal/cty"
)

const TypeVariable = "variable"

// Variable defines a variable which can be referenced by resources
type Variable struct {
	types.ResourceBase `xcl:",remain"`
	Default            cty.Value `xcl:"default" json:"default"`                            // default value for a variable
	Description        string    `xcl:"description,optional" json:"description,omitempty"` // description of the variable
}
