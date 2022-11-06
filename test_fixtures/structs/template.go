package structs

import "github.com/shipyard-run/hclconfig/types"

// TypeTemplate is the resource string for a Template resource
const TypeTemplate types.ResourceType = "template"

// Template allows the process of user defined templates
type Template struct {
	types.ResourceInfo `hcl:",remain" mapstructure:",squash"`

	Depends []string `hcl:"depends_on,optional" json:"depends,omitempty"`

	Source       string                 `hcl:"source" json:"source"`                // Source template to be processed as string
	Destination  string                 `hcl:"destination" json:"destination"`      // Desintation filename to write
	Vars         interface{}            `hcl:"vars,optional" json:"vars,omitempty"` // Variables to be processed in the template
	InternalVars map[string]interface{} // stores a converted go type version of the hcl.Value types
}

// New creates a new Nomad job config resource, implements Resource New method
func (t *Template) New(name string) types.Resource {
	return &Template{ResourceInfo: types.ResourceInfo{Name: name, Type: TypeTemplate, Status: types.PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (t *Template) Info() *types.ResourceInfo {
	return &t.ResourceInfo
}

func (t *Template) Parse(file string) error {
	return nil
}

func (t *Template) Process() error {
	return nil
}
