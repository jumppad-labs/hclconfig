package structs

import "github.com/instruqt/hclconfig/types"

// TypeNetwork is the string resource type for Network resources
const TypeNetwork = "network"

// Network defines a Docker network
type Network struct {
	types.ResourceBase `hcl:",remain"`

	Subnet string `hcl:"subnet" cty:"subnet" json:"subnet"`
}

func (c *Network) Process() error {
	return nil
}
