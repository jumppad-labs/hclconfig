package structs

import "github.com/shipyard-run/hclconfig/types"

// TypeNetwork is the string resource type for Network resources
const TypeNetwork types.ResourceType = "network"

// Network defines a Docker network
type Network struct {
	types.ResourceInfo `hcl:",remain" mapstructure:",squash"`

	Subnet string `hcl:"subnet" json:"subnet"`
}

// New creates a new Nomad job config resource, implements Resource New method
func (n *Network) New(name string) types.Resource {
	return &Network{ResourceInfo: types.ResourceInfo{Name: name, Type: TypeNetwork, Status: types.PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (n *Network) Info() *types.ResourceInfo {
	return &n.ResourceInfo
}

func (n *Network) Parse(file string) {
}
