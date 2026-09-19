package registry

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// PluginRegistry manages all resource types (builtin, registered and plugin-based) and can create resource instances
type PluginRegistry struct {
	builtinTypes    types.RegisteredTypes
	registeredTypes types.RegisteredTypes // plain Go types registered without a plugin
	pluginHosts     []plugins.PluginHost
	logger          logger.Logger
}

// NewPluginRegistry creates a new plugin registry with builtin types
func NewPluginRegistry(logger logger.Logger) *PluginRegistry {
	return &PluginRegistry{
		builtinTypes:    resources.DefaultResources(),
		registeredTypes: types.RegisteredTypes{},
		pluginHosts:     []plugins.PluginHost{},
		logger:          logger,
	}
}

// RegisterType registers a plain Go type as a configuration block type,
// without a plugin or provider. Blocks of the type are decoded into new
// instances of the Go type, take part in references and state, and never
// trigger a provider call.
//
// resource must be a pointer to a struct that embeds types.ResourceBase.
// Registering a name that is already provided by a builtin, a registered type
// or a plugin returns a *TypeNameClashError, and leaves the registry unchanged.
func (r *PluginRegistry) RegisterType(name string, resource any) error {
	value := reflect.ValueOf(resource)
	if resource == nil || value.Kind() != reflect.Ptr || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase", name)
	}

	if _, err := types.GetMeta(resource); err != nil {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase: %w", name, err)
	}

	if err := r.checkTypeName(name); err != nil {
		return err
	}

	r.registeredTypes[name] = resource

	return nil
}

// IsRegisteredType returns true when name was registered with RegisterType.
// Builtin and plugin types are not registered types.
func (r *PluginRegistry) IsRegisteredType(name string) bool {
	_, ok := r.registeredTypes[name]
	return ok
}

// CreateResource creates a new resource instance of the specified type and name
// It tries builtin types, then registered types, then falls back to plugin types
// Returns any to accommodate builtin, registered and schema-generated resources
func (r *PluginRegistry) CreateResource(resourceType, resourceName string) (any, error) {
	// First try builtin types
	if resource, err := r.builtinTypes.CreateResource(resourceType, resourceName); err == nil {
		return resource, nil
	}

	// Then try registered types, which are created as the registered Go type
	if resource, err := r.registeredTypes.CreateResource(resourceType, resourceName); err == nil {
		return resource, nil
	}

	// Then try plugin types
	return r.createResourceFromPlugins(resourceType, resourceName)
}

// checkTypeName returns a *TypeNameClashError when name is already provided
// by a builtin, a registered type or a loaded plugin
func (r *PluginRegistry) checkTypeName(name string) error {
	if _, ok := r.builtinTypes[name]; ok {
		return &TypeNameClashError{Name: name, Existing: "builtin"}
	}

	if _, ok := r.registeredTypes[name]; ok {
		return &TypeNameClashError{Name: name, Existing: "registered type"}
	}

	for _, host := range r.pluginHosts {
		for _, t := range host.GetTypes() {
			if t.Type == "resource" && t.SubType == name {
				return &TypeNameClashError{Name: name, Existing: "plugin"}
			}
		}
	}

	return nil
}

// checkHostTypes checks every resource type a new plugin host provides
// against the names already in the registry, and against the other types the
// same host provides. It returns every clash joined together.
func (r *PluginRegistry) checkHostTypes(host plugins.PluginHost) error {
	var clashes []error
	seen := map[string]bool{}

	for _, t := range host.GetTypes() {
		if t.Type != "resource" {
			continue
		}

		if seen[t.SubType] {
			clashes = append(clashes, &TypeNameClashError{Name: t.SubType, Existing: "the same plugin"})
			continue
		}
		seen[t.SubType] = true

		if err := r.checkTypeName(t.SubType); err != nil {
			clashes = append(clashes, err)
		}
	}

	return errors.Join(clashes...)
}

// createResourceFromPlugins attempts to create a resource using registered plugins
func (r *PluginRegistry) createResourceFromPlugins(resourceType, resourceName string) (any, error) {
	// Create type mapping for proper type creation
	typeMapping := map[string]reflect.Type{
		"types.Meta":         reflect.TypeOf(types.Meta{}),
		"types.ResourceBase": reflect.TypeOf(types.ResourceBase{}),
		"cty.Value":          reflect.TypeOf(cty.Value{}), // Treat cty.Value as interface{}
	}

	// Iterate through all plugin hosts
	for _, host := range r.pluginHosts {
		pluginTypes := host.GetTypes()

		// Look for a matching type
		for _, t := range pluginTypes {
			if t.Type == "resource" && t.SubType == resourceType {
				// Create a resource instance from the schema
				rawResource, err := schema.CreateInstanceFromSchema(t.Schema, typeMapping)
				if err != nil {
					return nil, fmt.Errorf("failed to create resource from schema for type %s: %w", resourceType, err)
				}

				meta, err := types.GetMeta(rawResource)
				if err != nil {
					panic(fmt.Sprintf("resource does not have ResourceBase embedded: %T", rawResource))
				}

				meta.Name = resourceName
				meta.Type = resourceType
				meta.Properties = make(map[string]any)

				return rawResource, nil
			}
		}
	}

	// No plugin found for this resource type
	return nil, fmt.Errorf("resource type %s not found in any registered plugin", resourceType)
}

// GetProvider finds the provider adapter for a given resource
// Returns nil if the resource type is a builtin type (not handled by plugins)
func (r *PluginRegistry) GetProvider(resource any) plugins.ProviderAdapter {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return nil // Skip resources without ResourceBase
	}
	resourceType := meta.Type

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

	// Reject the plugin when any of its types clash with a known type name
	if err := r.checkHostTypes(host); err != nil {
		host.Stop()
		return err
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

	// Reject the plugin when any of its types clash with a known type name,
	// types are only known once the plugin has started
	if err := r.checkHostTypes(host); err != nil {
		host.Stop()
		return fmt.Errorf("plugin %s: %w", pluginPath, err)
	}

	// Add to the list of plugin hosts
	r.pluginHosts = append(r.pluginHosts, host)

	return nil
}

// DiscoverAndLoadPlugins finds and loads all plugins from configured directories
func (r *PluginRegistry) DiscoverAndLoadPlugins(logger logger.Logger, directories []string, pattern string) error {
	pd := NewPluginDiscovery(directories, pattern, logger)

	// Discover plugins
	pluginPaths, err := pd.DiscoverPlugins()
	if err != nil {
		return fmt.Errorf("plugin discovery failed: %w", err)
	}

	// Track loading results
	var loadErrors []string
	var clashErrors []error
	successCount := 0

	// Load each discovered plugin
	for _, pluginPath := range pluginPaths {
		if err := r.RegisterPluginWithPath(pluginPath); err != nil {
			var clash *TypeNameClashError
			if errors.As(err, &clash) {
				clashErrors = append(clashErrors, err)
				logger.Error(fmt.Sprintf("Rejected plugin %s: %v", pluginPath, err))
				continue
			}

			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", pluginPath, err))
			logger.Error(fmt.Sprintf("Failed to load plugin %s: %v", pluginPath, err))
		} else {
			successCount++
			logger.Info(fmt.Sprintf("Successfully loaded plugin: %s", pluginPath))
		}
	}

	// Log summary
	if successCount > 0 {
		logger.Info(fmt.Sprintf("Plugin discovery complete: %d plugins loaded successfully", successCount))
	}

	if len(loadErrors) > 0 {
		logger.Warn(fmt.Sprintf("Plugin discovery warnings: %d plugins failed to load", len(loadErrors)))
	}

	// Type name clashes always fail discovery, even when other plugins loaded
	if len(clashErrors) > 0 {
		return errors.Join(clashErrors...)
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

// TODO: Rebuild CastResource without GenericResource

// CastResourceTo attempts to cast a resource to a specific concrete type
// This is a type-safe way to get strongly-typed resources from the registry
func CastResourceTo[T any](registry *PluginRegistry, resource any) (T, error) {
	var zero T

	// Try direct type assertion
	if concrete, ok := resource.(T); ok {
		return concrete, nil
	}

	return zero, fmt.Errorf("resource cannot be cast to requested type")
}

// GetProviderForResource gets the provider for any resource that embeds ResourceBase
func (r *PluginRegistry) GetProviderForResource(resource any) plugins.ProviderAdapter {
	meta, err := types.GetMeta(resource)
	if err != nil {
		panic(fmt.Sprintf("resource does not have ResourceBase embedded: %T", resource))
	}

	resourceType := meta.Type

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

	return nil
}
