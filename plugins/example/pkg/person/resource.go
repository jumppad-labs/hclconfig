package person

import "github.com/jumppad-labs/hclconfig/types"

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
}
