package resources

import "github.com/jumppad-labs/xcl/types"

// DefaultResources is a collection of the default config resources
func DefaultResources() types.RegisteredTypes {
	return types.RegisteredTypes{
		"variable": &Variable{},
		"output":   &Output{},
		"module":   &Module{},
		"root":     &Root{},
	}
}
