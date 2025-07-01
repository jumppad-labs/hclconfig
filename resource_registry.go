package hclconfig

import (
	"fmt"
	"reflect"

	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/types"
)

// ResourceRegistry manages all resource types (builtin and plugin-based) and can create resource instances
type ResourceRegistry struct {
	builtinTypes types.RegisteredTypes
	pluginHosts  []plugins.PluginHost
}

// NewResourceRegistry creates a new resource registry with builtin types and plugin hosts
func NewResourceRegistry(pluginHosts []plugins.PluginHost) *ResourceRegistry {
	return &ResourceRegistry{
		builtinTypes: resources.DefaultResources(),
		pluginHosts:  pluginHosts,
	}
}

// CreateResource creates a new resource instance of the specified type and name
// It first tries builtin types, then falls back to plugin types
func (r *ResourceRegistry) CreateResource(resourceType, resourceName string) (types.Resource, error) {
	// First try builtin types
	if resource, err := r.builtinTypes.CreateResource(resourceType, resourceName); err == nil {
		return resource, nil
	}

	// Then try plugin types
	return r.createResourceFromPlugins(resourceType, resourceName)
}

// createResourceFromPlugins attempts to create a resource using registered plugins
func (r *ResourceRegistry) createResourceFromPlugins(resourceType, resourceName string) (types.Resource, error) {
	// Iterate through all plugin hosts
	for _, host := range r.pluginHosts {
		pluginTypes := host.GetTypes()

		// Look for a matching type
		for _, t := range pluginTypes {
			if t.Type == "resource" && t.SubType == resourceType {
				// Found a matching plugin type, create resource from concrete type
				if t.ConcreteType == nil {
					return nil, fmt.Errorf("plugin type %s has no concrete type", resourceType)
				}

				// Create a new instance of the concrete type using reflection
				ptr := reflect.New(reflect.TypeOf(t.ConcreteType).Elem())
				resource := ptr.Interface().(types.Resource)

				// Initialize the resource metadata
				resource.Metadata().Name = resourceName
				resource.Metadata().Type = resourceType
				resource.Metadata().Properties = map[string]any{}

				return resource, nil
			}
		}
	}

	// No plugin found for this resource type
	return nil, fmt.Errorf("resource type %s not found in any registered plugin", resourceType)
}