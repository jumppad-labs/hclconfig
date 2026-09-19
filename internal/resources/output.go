package resources

import (
	"github.com/jumppad-labs/xcl/types"
	"github.com/jumppad-labs/xcl/internal/cty"
)

const TypeOutput = "output"

// Output defines an output variable which can be set by a module
type Output struct {
	types.ResourceBase `xcl:",remain"`

	CtyValue    cty.Value `xcl:"value,optional"` // value of the output
	Value       any       `json:"value"`
	Description string    `xcl:"description,optional" json:"description,omitempty"` // description for the output
}
