package types

const TypeVariable = "variable"

// Output defines an output variable which can be set by a module
type Variable struct {
	ResourceBase `hcl:",remain"`
	Default      interface{} `hcl:"default" json:"default"`                            // default value for a variable
	Description  string      `hcl:"description,optional" json:"description,omitempty"` // description of the variable
}
