package resources

import "github.com/jumppad-labs/xcl/types"

// TypeModule is the resource string for a Module resource
const TypeRoot = "root"

// Module allows Shipyard configuration to be imported from external folder or
// GitHub repositories
type Root struct {
	types.ResourceBase `hcl:",remain"`
}
