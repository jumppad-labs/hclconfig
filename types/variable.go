package types

const TypeVariable ResourceType = "variable"

// Output defines an output variable which can be set by a module
type Variable struct {
	ResourceInfo `mapstructure:",squash"`
	Default      interface{} `hcl:"default" json:"default"`                            // default value for a variable
	Description  string      `hcl:"description,optional" json:"description,omitempty"` // description of the variable
}

// New creates a new Nomad job config resource, implements Resource New method
func (t *Variable) New(name string) Resource {
	return &Variable{ResourceInfo: ResourceInfo{Name: name, Type: TypeVariable, Status: PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (t *Variable) Info() *ResourceInfo {
	return &t.ResourceInfo
}

func (t *Variable) Parse(file string) error {
	return nil
}

func (t *Variable) Process() error {
	return nil
}
