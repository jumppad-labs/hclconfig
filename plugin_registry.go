package hclconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/types"
)

// PluginRegistry manages all resource types (builtin and plugin-based) and can create resource instances
type PluginRegistry struct {
	builtinTypes types.RegisteredTypes
	pluginHosts  []plugins.PluginHost
	logger       logger.Logger
}

// NewPluginRegistry creates a new plugin registry with builtin types
func NewPluginRegistry(logger logger.Logger) *PluginRegistry {
	return &PluginRegistry{
		builtinTypes: resources.DefaultResources(),
		pluginHosts:  []plugins.PluginHost{},
		logger:       logger,
	}
}

// CreateResource creates a new resource instance of the specified type and name
// It first tries builtin types, then falls back to plugin types
func (r *PluginRegistry) CreateResource(resourceType, resourceName string) (types.Resource, error) {
	// First try builtin types
	if resource, err := r.builtinTypes.CreateResource(resourceType, resourceName); err == nil {
		return resource, nil
	}

	// Then try plugin types
	return r.createResourceFromPlugins(resourceType, resourceName)
}

// createResourceFromPlugins attempts to create a resource using registered plugins
func (r *PluginRegistry) createResourceFromPlugins(resourceType, resourceName string) (types.Resource, error) {
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

// GetProvider finds the provider adapter for a given resource
// Returns nil if the resource type is a builtin type (not handled by plugins)
func (r *PluginRegistry) GetProvider(resource types.Resource) plugins.ProviderAdapter {
	resourceType := resource.Metadata().Type
	
	// Check if it's a builtin type
	if _, err := r.builtinTypes.CreateResource(resourceType, "dummy"); err == nil {
		// This is a builtin type, not handled by plugins
		return nil
	}
	
	// Search through plugin hosts
	for _, host := range r.pluginHosts {
		pluginTypes := host.GetTypes()
		
		for _, t := range pluginTypes {
			if t.Type == "resource" && t.SubType == resourceType {
				// Found the matching plugin type
				return t.Adapter
			}
		}
	}
	
	// No plugin found for this resource type
	return nil
}

// RegisterPlugin registers an in-process plugin with the registry
func (r *PluginRegistry) RegisterPlugin(plugin plugins.Plugin) error {
	// Create a DirectPluginHost for the in-process plugin
	host, err := plugins.NewDirectPluginHost(r.logger, nil, plugin)
	if err != nil {
		return fmt.Errorf("failed to create plugin host: %w", err)
	}

	// Add to the list of plugin hosts
	r.pluginHosts = append(r.pluginHosts, host)

	return nil
}

// RegisterPluginWithPath registers an external plugin from a file path
func (r *PluginRegistry) RegisterPluginWithPath(pluginPath string) error {
	// Create a GRPC plugin host for the external plugin
	host := plugins.NewGRPCPluginHost(r.logger, nil)

	// Start the external plugin
	err := host.Start(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to start external plugin %s: %w", pluginPath, err)
	}

	// Add to the list of plugin hosts
	r.pluginHosts = append(r.pluginHosts, host)

	return nil
}

// DiscoverAndLoadPlugins finds and loads all plugins from configured directories
func (r *PluginRegistry) DiscoverAndLoadPlugins(options *ParserOptions) error {
	if !options.AutoDiscoverPlugins {
		return nil
	}

	// Create plugin discovery instance
	var loggerFunc func(string)
	if options.Logger != nil {
		loggerFunc = func(msg string) {
			options.Logger.Info(msg)
		}
	}
	pd := NewPluginDiscovery(options.PluginDirectories, options.PluginNamePattern, loggerFunc)

	// Discover plugins
	pluginPaths, err := pd.DiscoverPlugins()
	if err != nil {
		return fmt.Errorf("plugin discovery failed: %w", err)
	}

	// Track loading results
	var loadErrors []string
	successCount := 0

	// Load each discovered plugin
	for _, pluginPath := range pluginPaths {
		if err := r.RegisterPluginWithPath(pluginPath); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", pluginPath, err))
			if options.Logger != nil {
				options.Logger.Error(fmt.Sprintf("Failed to load plugin %s: %v", pluginPath, err))
			}
		} else {
			successCount++
			if options.Logger != nil {
				options.Logger.Info(fmt.Sprintf("Successfully loaded plugin: %s", pluginPath))
			}
		}
	}

	// Log summary
	if options.Logger != nil {
		if successCount > 0 {
			options.Logger.Info(fmt.Sprintf("Plugin discovery complete: %d plugins loaded successfully", successCount))
		}
		if len(loadErrors) > 0 {
			options.Logger.Warn(fmt.Sprintf("Plugin discovery warnings: %d plugins failed to load", len(loadErrors)))
		}
	}

	// Only return error if all plugins failed to load and we found some
	if len(loadErrors) > 0 && successCount == 0 && len(pluginPaths) > 0 {
		return fmt.Errorf("all plugin loads failed: %s", strings.Join(loadErrors, "; "))
	}

	return nil
}

// GetPluginHosts returns the list of plugin hosts (for compatibility)
func (r *PluginRegistry) GetPluginHosts() []plugins.PluginHost {
	return r.pluginHosts
}

// CastResource attempts to cast a generic resource to its concrete type
// This is useful when loading resources from state storage
func (r *PluginRegistry) CastResource(resource types.Resource) (types.Resource, error) {
	// If it's already a concrete type (not GenericResource), return as-is
	if _, isGeneric := resource.(*GenericResource); !isGeneric {
		return resource, nil
	}
	
	genericResource := resource.(*GenericResource)
	resourceType := genericResource.Metadata().Type
	resourceName := genericResource.Metadata().Name
	
	// Create the concrete resource type
	concreteResource, err := r.CreateResource(resourceType, resourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create concrete resource: %w", err)
	}
	
	// Copy the metadata
	*concreteResource.Metadata() = genericResource.Meta
	
	// Copy the base fields
	concreteResource.SetDisabled(genericResource.GetDisabled())
	concreteResource.SetDependencies(genericResource.GetDependencies())
	
	// For now, return the concrete resource with copied metadata
	// TODO: In a future enhancement, we could use reflection to copy the additional fields
	// from genericResource.Data into the concrete resource's fields
	
	return concreteResource, nil
}

// CastResourceTo attempts to cast a resource to a specific concrete type
// This is a type-safe way to get strongly-typed resources from the registry
func CastResourceTo[T types.Resource](registry *PluginRegistry, resource types.Resource) (T, error) {
	var zero T
	
	// First try direct type assertion
	if concrete, ok := resource.(T); ok {
		return concrete, nil
	}
	
	// If it's a generic resource, try casting through the registry
	if _, isGeneric := resource.(*GenericResource); isGeneric {
		cast, err := registry.CastResource(resource)
		if err != nil {
			return zero, err
		}
		
		if concrete, ok := cast.(T); ok {
			return concrete, nil
		}
	}
	
	return zero, fmt.Errorf("resource cannot be cast to requested type")
}