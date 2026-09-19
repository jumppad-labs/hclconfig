package person

import "github.com/jumppad-labs/xcl/types"

// Person is an example resource that implements types.Resource
type Person struct {
	types.ResourceBase `xcl:",remain"`

	// Basic person fields
	FirstName   string `xcl:"first_name" json:"first_name"`
	LastName    string `xcl:"last_name" json:"last_name"`
	Age         int    `xcl:"age,optional" json:"age,omitempty"`
	Email       string `xcl:"email,optional" json:"email,omitempty"`
	Address     string `xcl:"address,optional" json:"address,omitempty"`
	Description string `xcl:"description,optional" json:"description,omitempty"`

	// PersonID is owned by the provider: it is set when the person is created,
	// can not be set in configuration, and is carried over by xcl on every apply
	PersonID string `xcl:"person_id,optional,computed" json:"person_id,omitempty"`
}
