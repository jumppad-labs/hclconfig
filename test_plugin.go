package hclconfig

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/hclconfig/internal/test_fixtures/embedded"
	"github.com/jumppad-labs/hclconfig/types"
)

// TestPluginConfig represents the configuration for the test plugin
type TestPluginConfig struct {
	// Empty config for testing
}

// TestPlugin provides test resource types for testing the parser
type TestPlugin struct {
	plugins.PluginBase
	RefreshedResources []string // Track all refreshed resource names
	CreatedResources   []string // Track all created resource names
	DestroyedResources []string // Track all destroyed resource names
	UpdatedResources   []string // Track all updated resource names
	ChangedCalls       []string // Track all changed checks (format: "oldID->newID")
	
	// Error configuration maps
	CreateErrors  map[string]error // Maps resource ID to error for Create operations
	DestroyErrors map[string]error // Maps resource ID to error for Destroy operations
	UpdateErrors  map[string]error // Maps resource ID to error for Update operations
	RefreshErrors map[string]error // Maps resource ID to error for Refresh operations
	ChangedErrors map[string]error // Maps resource ID to error for Changed operations
}

// GetConfigType is now automatically provided by PluginBase

// Ensure TestPlugin implements Plugin interface
var _ plugins.Plugin = (*TestPlugin)(nil)

// GetRefreshedResources returns the list of resource names that were refreshed
func (p *TestPlugin) GetRefreshedResources() []string {
	return p.RefreshedResources
}

// GetCreatedResources returns the list of resource names that were created
func (p *TestPlugin) GetCreatedResources() []string {
	return p.CreatedResources
}

// GetDestroyedResources returns the list of resource names that were destroyed
func (p *TestPlugin) GetDestroyedResources() []string {
	return p.DestroyedResources
}

// GetUpdatedResources returns the list of resource names that were updated
func (p *TestPlugin) GetUpdatedResources() []string {
	return p.UpdatedResources
}

// GetChangedCalls returns the list of changed checks that were made
func (p *TestPlugin) GetChangedCalls() []string {
	return p.ChangedCalls
}

// SetCreateError configures an error to be returned when creating a resource with the given ID
func (p *TestPlugin) SetCreateError(resourceID string, err error) {
	if p.CreateErrors == nil {
		p.CreateErrors = make(map[string]error)
	}
	p.CreateErrors[resourceID] = err
}

// SetDestroyError configures an error to be returned when destroying a resource with the given ID
func (p *TestPlugin) SetDestroyError(resourceID string, err error) {
	if p.DestroyErrors == nil {
		p.DestroyErrors = make(map[string]error)
	}
	p.DestroyErrors[resourceID] = err
}

// SetUpdateError configures an error to be returned when updating a resource with the given ID
func (p *TestPlugin) SetUpdateError(resourceID string, err error) {
	if p.UpdateErrors == nil {
		p.UpdateErrors = make(map[string]error)
	}
	p.UpdateErrors[resourceID] = err
}

// SetRefreshError configures an error to be returned when refreshing a resource with the given ID
func (p *TestPlugin) SetRefreshError(resourceID string, err error) {
	if p.RefreshErrors == nil {
		p.RefreshErrors = make(map[string]error)
	}
	p.RefreshErrors[resourceID] = err
}

// SetChangedError configures an error to be returned when checking if a resource with the given ID has changed
func (p *TestPlugin) SetChangedError(resourceID string, err error) {
	if p.ChangedErrors == nil {
		p.ChangedErrors = make(map[string]error)
	}
	p.ChangedErrors[resourceID] = err
}

// ClearErrors clears all configured errors
func (p *TestPlugin) ClearErrors() {
	p.CreateErrors = make(map[string]error)
	p.DestroyErrors = make(map[string]error)
	p.UpdateErrors = make(map[string]error)
	p.RefreshErrors = make(map[string]error)
	p.ChangedErrors = make(map[string]error)
}

// Init initializes the test plugin with test resource types
func (p *TestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Initialize all tracking slices
	p.RefreshedResources = []string{}
	p.CreatedResources = []string{}
	p.DestroyedResources = []string{}
	p.UpdatedResources = []string{}
	p.ChangedCalls = []string{}
	
	// Initialize error maps
	p.CreateErrors = make(map[string]error)
	p.DestroyErrors = make(map[string]error)
	p.UpdateErrors = make(map[string]error)
	p.RefreshErrors = make(map[string]error)
	p.ChangedErrors = make(map[string]error)
	
	// Register Container resource
	containerResource := &structs.Container{}
	containerProvider := &TestResourceProvider[*structs.Container]{plugin: p}
	var config TestPluginConfig
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"container",
		containerResource,
		containerProvider,
		config,
	)
	if err != nil {
		return err
	}

	sidecarResource := &structs.Sidecar{}
	sidecarProvider := &TestResourceProvider[*structs.Sidecar]{plugin: p}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"sidecar",
		sidecarResource,
		sidecarProvider,
		config,
	)
	if err != nil {
		return err
	}

	// Register Network resource
	networkResource := &structs.Network{}
	networkProvider := &TestResourceProvider[*structs.Network]{plugin: p}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"network",
		networkResource,
		networkProvider,
		config,
	)
	if err != nil {
		return err
	}

	// Register Template resource
	templateResource := &structs.Template{}
	templateProvider := &TestResourceProvider[*structs.Template]{plugin: p}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"template",
		templateResource,
		templateProvider,
		config,
	)
	if err != nil {
		return err
	}

	return nil
}

// TestResourceProvider is a generic test provider for any resource type
type TestResourceProvider[T types.Resource] struct {
	logger         logger.Logger
	state          plugins.State
	functions      plugins.ProviderFunctions
	plugin         *TestPlugin // Reference to parent plugin for tracking
}

// Init initializes the test provider
func (p *TestResourceProvider[T]) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger, config TestPluginConfig) error {
	p.state = state
	p.functions = functions
	p.logger = logger
	return nil
}

// Create tracks the resource name for testing
func (p *TestResourceProvider[T]) Create(ctx context.Context, resource T) (T, error) {
	// Always track the resource name when create is called first
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.CreatedResources = append(p.plugin.CreatedResources, res.Metadata().ID)
		}
	}
	
	// Then check if an error is configured for this resource ID
	if p.plugin != nil {
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			if err, exists := p.plugin.CreateErrors[res.Metadata().ID]; exists && err != nil {
				return resource, err
			}
		}
	}
	return resource, nil
}

// Destroy tracks the resource name for testing
func (p *TestResourceProvider[T]) Destroy(ctx context.Context, resource T, force bool) error {
	// Always track the resource name when destroy is called first
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.DestroyedResources = append(p.plugin.DestroyedResources, res.Metadata().ID)
		}
	}
	
	// Then check if an error is configured for this resource ID
	if p.plugin != nil {
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			if err, exists := p.plugin.DestroyErrors[res.Metadata().ID]; exists && err != nil {
				return err
			}
		}
	}
	return nil
}

// Refresh tracks the resource name for testing
func (p *TestResourceProvider[T]) Refresh(ctx context.Context, resource T) error {
	// Always track the resource name when refresh is called first
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.RefreshedResources = append(p.plugin.RefreshedResources, res.Metadata().ID)
		}
	}
	
	// Then check if an error is configured for this resource ID
	if p.plugin != nil {
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			if err, exists := p.plugin.RefreshErrors[res.Metadata().ID]; exists && err != nil {
				return err
			}
		}
	}
	return nil
}

// Update tracks the resource name for testing
func (p *TestResourceProvider[T]) Update(ctx context.Context, resource T) error {
	// Always track the resource name when update is called first
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.UpdatedResources = append(p.plugin.UpdatedResources, res.Metadata().ID)
		}
	}
	
	// Then check if an error is configured for this resource ID
	if p.plugin != nil {
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			if err, exists := p.plugin.UpdateErrors[res.Metadata().ID]; exists && err != nil {
				return err
			}
		}
	}
	return nil
}

// Changed tracks the comparison and always returns false for testing
func (p *TestResourceProvider[T]) Changed(ctx context.Context, old T, new T) (bool, error) {
	// Always track the changed check when it's called first
	if p.plugin != nil {
		// Use type assertion to check if resources implement the Resource interface
		oldRes, oldOk := any(old).(types.Resource)
		newRes, newOk := any(new).(types.Resource)
		
		if oldOk && newOk && oldRes != nil && newRes != nil {
			// Format: "oldID->newID"
			call := fmt.Sprintf("%s->%s", oldRes.Metadata().ID, newRes.Metadata().ID)
			p.plugin.ChangedCalls = append(p.plugin.ChangedCalls, call)
		}
	}
	
	// Then check if an error is configured for this resource ID (using new resource ID)
	if p.plugin != nil {
		oldRes, oldOk := any(old).(types.Resource)
		newRes, newOk := any(new).(types.Resource)
		
		if oldOk && newOk && oldRes != nil && newRes != nil {
			if err, exists := p.plugin.ChangedErrors[newRes.Metadata().ID]; exists && err != nil {
				return false, err
			}
		}
	}
	return false, nil
}

// Functions returns no functions
func (p *TestResourceProvider[T]) Functions() plugins.ProviderFunctions {
	return p.functions
}


// EmbeddedTestPlugin provides embedded test resource types
type EmbeddedTestPlugin struct {
	plugins.PluginBase
}

// GetConfigType is now automatically provided by PluginBase

// Ensure EmbeddedTestPlugin implements Plugin interface
var _ plugins.Plugin = (*EmbeddedTestPlugin)(nil)

// Init initializes the embedded test plugin
func (p *EmbeddedTestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Register Container resource
	containerResource := &embedded.Container{}
	containerProvider := &TestResourceProvider[*embedded.Container]{}
	var config TestPluginConfig
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"container",
		containerResource,
		containerProvider,
		config,
	)
	if err != nil {
		return err
	}

	// Register Sidecar resource
	sidecarResource := &embedded.Sidecar{}
	sidecarProvider := &TestResourceProvider[*embedded.Sidecar]{}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"sidecar",
		sidecarResource,
		sidecarProvider,
		config,
	)
	if err != nil {
		return err
	}

	return nil
}