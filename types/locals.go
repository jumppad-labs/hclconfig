package types

import "github.com/zclconf/go-cty/cty"

const TypeLocal = "local"

// Output defines an output variable which can be set by a module
type Local struct {
	ResourceBase `hcl:",remain"`

	CtyValue cty.Value   `hcl:"value,optional"` // value of the output
	Value    interface{} `json:"value"`
}
