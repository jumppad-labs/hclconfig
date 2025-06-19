package plugins

import "github.com/jumppad-labs/hclconfig/types"

// State manages the persistent state of resources.
type State interface {
	// Get retrieves the current state of a resource by its key.
	// It returns an error if the resource does not exist or access is denied.
	Get(key string) (types.Resource, error)

	// Find retrieves resources matching a specific pattern.
	// For example, "resource.container.*" would return all container resources.
	// It returns an error if no resources match or access is denied.
	Find(pattern string) ([]types.Resource, error)
}
