package types

const TypeOutput = "output"

// Output defines an output variable which can be set by a module
type Output struct {
	ResourceMetadata `hcl:",remain"`

	Value string `hcl:"value,optional" json:"value,omitempty"` // command to use when starting the container
}
