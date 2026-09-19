package person

import "github.com/jumppad-labs/xcl/types"

// Person is an example resource that implements types.Resource
type Person struct {
	types.ResourceBase `hcl:",remain"`

	// Basic person fields
	FirstName   string `hcl:"first_name" json:"first_name"`
	LastName    string `hcl:"last_name" json:"last_name"`
	Age         int    `hcl:"age,optional" json:"age,omitempty"`
	Email       string `hcl:"email,optional" json:"email,omitempty"`
	Address     string `hcl:"address,optional" json:"address,omitempty"`
	Description string `hcl:"description,optional" json:"description,omitempty"`

	// PersonID is owned by the provider: it is set when the person is created,
	// can not be set in configuration, and is carried over by xcl on every apply
	PersonID string `hcl:"person_id,optional" json:"person_id,omitempty" xcl:"computed"`
}
