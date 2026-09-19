package structs

import (
	"github.com/jumppad-labs/xcl/types"
	"github.com/jumppad-labs/xcl/internal/cty"
)

// TypeTemplate is the resource string for a Template resource
const TypeTemplate = "template"

// Template allows the process of user defined templates
type Template struct {
	types.ResourceBase `xcl:",remain"`

	Depends []string `xcl:"depends_on,optional" json:"depends,omitempty"`

	Source      string    `xcl:"source" json:"source"`                // Source template to be processed as string
	Destination string    `xcl:"destination" json:"destination"`      // Destination filename to write
	Vars        cty.Value `xcl:"vars,optional" json:"vars,omitempty"` // Variables to be processed in the template
	//InternalVars map[string]any // stores a converted go type version of the hcl.Value types
	AppendFile bool `xcl:"append_file,optional" json:"append_file,omitempty"`

	Inner *Thing `xcl:"inner" json:"inner,omitempty"`
}

type Thing struct {
	InnerString string `xcl:"inner_string" json:"inner_string"`
	InnerInt    int    `xcl:"inner_int" json:"inner_int"`
}
