package resources

import (
	"github.com/instruqt/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)

const TypeLocal = "local"

// Output defines an output variable which can be set by a module
type Local struct {
	types.ResourceBase `hcl:",remain"`

	CtyValue cty.Value `hcl:"value,optional"` // value of the output
	Value    any       `json:"value"`
}
