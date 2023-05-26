package structs

import "github.com/jumppad-labs/hclconfig/types"

// TypeNetwork is the string resource type for Network resources
const TypeNetwork = "network"

// Network defines a Docker network
type Network struct {
	types.ResourceMetadata `hcl:",remain"`

	Subnet string `hcl:"subnet" json:"subnet"`
}

func (c *Network) Process() error {
	return nil
}
