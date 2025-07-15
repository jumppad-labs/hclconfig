package resources

import (
	"reflect"

	"github.com/jumppad-labs/hclconfig/types"
)

const TypeProvider = "provider"

// Provider represents a provider configuration resource, similar to Variable
// It extends ResourceBase to enable FQRN referencing like provider.name.config.property
type Provider struct {
	types.ResourceBase `hcl:",remain"`
	
	// Source is the provider source (e.g., "jumppad/containerd")
	Source string `hcl:"source"`
	
	// Version is the provider version constraint (e.g., "~> 1.0")
	Version string `hcl:"version,optional"`
	
	// Config will be populated manually during parsing (config block is dynamic, not via HCL tags)
	Config interface{} `json:"config,omitempty"`
	
	// Internal fields for plugin management (not exposed via HCL)
	ConfigType   reflect.Type   `json:"-"`
	Initialized  bool           `json:"-"`
}