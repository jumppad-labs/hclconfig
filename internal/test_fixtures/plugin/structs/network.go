package structs

import "github.com/jumppad-labs/xcl/types"

// TypeNetwork is the string resource type for Network resources
const TypeNetwork = "network"

// Network defines a Docker network
type Network struct {
	types.ResourceBase `hcl:",remain"`

	Subnet string `hcl:"subnet" json:"subnet"`

	// ProviderID is set by the provider when the network is created
	ProviderID string `hcl:"provider_id,optional" json:"provider_id,omitempty" xcl:"computed"`

	// Observed is set by the provider when it reads the real network
	Observed string `hcl:"observed,optional" json:"observed,omitempty" xcl:"computed"`
}
