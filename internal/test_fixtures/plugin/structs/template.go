package structs

import (
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)

// TypeTemplate is the resource string for a Template resource
const TypeTemplate = "template"

// Template allows the process of user defined templates
type Template struct {
	types.ResourceBase `hcl:",remain"`

	Depends []string `hcl:"depends_on,optional" json:"depends,omitempty"`

	Source       string         `hcl:"source" json:"source"`                // Source template to be processed as string
	Destination  string         `hcl:"destination" json:"destination"`      // Destination filename to write
	Vars         cty.Value      `hcl:"vars,optional" json:"vars,omitempty"` // Variables to be processed in the template
	InternalVars map[string]any // stores a converted go type version of the hcl.Value types
	AppendFile   bool           `hcl:"append_file,optional" json:"append_file,omitempty"`
}

func (t *Template) Process() error {
	return nil
}
