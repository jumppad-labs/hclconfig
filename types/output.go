package types

import "github.com/zclconf/go-cty/cty"

const TypeOutput = "output"

// Output defines an output variable which can be set by a module
type Output struct {
	ResourceMetadata `hcl:",remain"`

	CtyValue    cty.Value   `hcl:"value,optional"` // value of the output
	Value       interface{} `json:"value"`
	Description string      `hcl:"description,optional" json:"description,omitempty"` // description for the output
}
