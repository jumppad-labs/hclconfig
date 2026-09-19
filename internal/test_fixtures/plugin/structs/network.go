package structs

import "github.com/jumppad-labs/xcl/types"

// TypeNetwork is the string resource type for Network resources
const TypeNetwork = "network"

// Network defines a Docker network
type Network struct {
	types.ResourceBase `xcl:",remain"`

	Subnet string `xcl:"subnet" json:"subnet"`

	// ProviderID is set by the provider when the network is created
	ProviderID string `xcl:"provider_id,optional,computed" json:"provider_id,omitempty"`

	// Observed is set by the provider when it reads the real network
	Observed string `xcl:"observed,optional,computed" json:"observed,omitempty"`
}
