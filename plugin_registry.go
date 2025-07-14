package hclconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/state"
	"github.com/jumppad-labs/hclconfig/types"
)


// PluginRegistry manages all resource types (builtin and plugin-based), can create resource instances,
// and manages provider configurations
type PluginRegistry struct {
	builtinTypes types.RegisteredTypes
	pluginHosts  []plugins.PluginHost
	logger       logger.Logger
	
	// Plugin and provider management
	// plugins: maps source identifiers (e.g., "jumppad/containerd") to plugin implementations
	// This allows finding which plugin handles a given source when registering providers
	plugins      map[string]plugins.Plugin  // source -> plugin mapping
	
	// providers: maps provider instance names (e.g., "docker", "podman") to configured provider instances
	// Each provider instance has a name, references a source plugin, and contains typed configuration
	providers    map[string]*resources.Provider // name -> configured provider instance
}

// NewPluginRegistry creates a new plugin registry with builtin types
func NewPluginRegistry(logger logger.Logger) *PluginRegistry {
	return &PluginRegistry{
		builtinTypes: resources.DefaultResources(),
		pluginHosts:  []plugins.PluginHost{},
		logger:       logger,
		plugins:      make(map[string]plugins.Plugin),
		providers:    make(map[string]*resources.Provider),
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

// GetProviderAdapter finds the provider adapter for a given resource
// Returns nil if the resource type is a builtin type (not handled by plugins)
func (r *PluginRegistry) GetProviderAdapter(resource types.Resource) plugins.ProviderAdapter {
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
	if _, isGeneric := resource.(*state.GenericResource); !isGeneric {
		return resource, nil
	}
	
	genericResource := resource.(*state.GenericResource)
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
	if _, isGeneric := resource.(*state.GenericResource); isGeneric {
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

// Provider Configuration Methods

// RegisterProvider registers a provider instance with its configuration
// This is called after the provider resource has been parsed to set up plugin-specific fields
func (r *PluginRegistry) RegisterProvider(provider *resources.Provider, ctx *hcl.EvalContext) error {
	// Validate required fields
	if provider.Metadata().Name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider.Source == "" {
		return fmt.Errorf("provider source cannot be empty")
	}

	// Check if provider name already exists
	if _, exists := r.providers[provider.Metadata().Name]; exists {
		return fmt.Errorf("provider '%s' is already defined", provider.Metadata().Name)
	}

	// Find plugin by source
	plugin, err := r.findPluginBySource(provider.Source)
	if err != nil {
		return fmt.Errorf("failed to find plugin for source '%s': %w", provider.Source, err)
	}

	// Get config type from plugin
	configType := plugin.GetConfigType()
	if configType == nil {
		return fmt.Errorf("plugin for source '%s' does not define a configuration type", provider.Source)
	}

	// Convert hcl.Body config to concrete type if config is provided
	if provider.Config != nil {
		if configBody, ok := provider.Config.(hcl.Body); ok {
			// Create an instance of the concrete config type
			configPtr := reflect.New(configType).Interface()
			
			// Decode the config body to the concrete type
			diags := gohcl.DecodeBody(configBody, ctx, configPtr)
			if diags.HasErrors() {
				return fmt.Errorf("failed to decode provider config: %s", diags.Error())
			}
			
			// Store the concrete config as a pointer
			provider.Config = configPtr
		}
	}

	// Set up plugin-specific fields
	provider.Plugin = plugin
	provider.ConfigType = configType
	provider.Initialized = false

	// Register this provider instance for quick lookup
	r.providers[provider.Metadata().Name] = provider
	return nil
}


// GetProvider returns a provider instance by name
func (r *PluginRegistry) GetProvider(name string) (*resources.Provider, error) {
	providerConfig, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' is not defined", name)
	}
	return providerConfig, nil
}

// GetDefaultProvider returns the default provider for a resource type
// Default provider name matches resource type (e.g., "container" provider for container resources)
func (r *PluginRegistry) GetDefaultProvider(resourceType string) (*resources.Provider, error) {
	return r.GetProvider(resourceType)
}

// ListProviders returns all registered provider names (excluding internal source mappings)
func (r *PluginRegistry) ListProviders() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		// Skip internal source mapping entries
		if !strings.HasPrefix(name, "_source_") {
			names = append(names, name)
		}
	}
	return names
}

// ResolveProviderForResource determines which provider should handle a resource
func (r *PluginRegistry) ResolveProviderForResource(resource interface{}) (*resources.Provider, error) {
	// Check if resource has explicit provider field
	if providerField, err := getProviderField(resource); err == nil && providerField != "" {
		return r.GetProvider(providerField)
	}

	// Fall back to default provider based on resource type
	resourceType := getResourceType(resource)
	return r.GetDefaultProvider(resourceType)
}

// RegisterPluginSource registers a plugin with a source identifier for provider configuration
func (r *PluginRegistry) RegisterPluginSource(source string, plugin plugins.Plugin) {
	// Store the plugin directly in the plugins map
	r.plugins[source] = plugin
}

// findPluginBySource finds a plugin by its source identifier
func (r *PluginRegistry) findPluginBySource(source string) (plugins.Plugin, error) {
	// Look in the dedicated plugins map first
	if plugin, exists := r.plugins[source]; exists {
		return plugin, nil
	}
	
	// Also look through existing providers to find matching source (for backwards compatibility)
	for _, provider := range r.providers {
		if provider.Source == source {
			return provider.Plugin, nil
		}
	}
	
	return nil, fmt.Errorf("plugin with source '%s' not found - use RegisterPluginSource to register it", source)
}

// Helper function to extract provider field from resource using reflection
func getProviderField(resource interface{}) (string, error) {
	val := reflect.ValueOf(resource)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return "", fmt.Errorf("resource is not a struct")
	}

	// Look for Provider field
	providerField := val.FieldByName("Provider")
	if !providerField.IsValid() {
		return "", fmt.Errorf("resource does not have Provider field")
	}

	if providerField.Kind() != reflect.String {
		return "", fmt.Errorf("Provider field is not a string")
	}

	return providerField.String(), nil
}

// Helper function to extract resource type
func getResourceType(resource interface{}) string {
	// This is a simplified implementation
	// In practice, you'd extract this from the resource's metadata
	val := reflect.TypeOf(resource)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val.Name()
}