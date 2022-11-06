package types

const TypeOutput ResourceType = "output"

// Output defines an output variable which can be set by a module
type Output struct {
	ResourceInfo `hcl:",remain" mapstructure:",squash"`

	Value string `hcl:"value,optional" json:"value,omitempty"` // command to use when starting the container
}

// New creates a new Nomad job config resource, implements Resource New method
func (t *Output) New(name string) Resource {
	return &Output{ResourceInfo: ResourceInfo{Name: name, Type: TypeOutput, Status: PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (t *Output) Info() *ResourceInfo {
	return &t.ResourceInfo
}

func (t *Output) Parse(file string) error {
	return nil
}

func (t *Output) Process() error {
	return nil
}
