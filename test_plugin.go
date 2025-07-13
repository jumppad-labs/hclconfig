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

// TestPlugin provides test resource types for testing the parser
type TestPlugin struct {
	plugins.PluginBase
	RefreshedResources []string // Track all refreshed resource names
	CreatedResources   []string // Track all created resource names
	DestroyedResources []string // Track all destroyed resource names
	UpdatedResources   []string // Track all updated resource names
	ChangedCalls       []string // Track all changed checks (format: "oldID->newID")
}

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

// Init initializes the test plugin with test resource types
func (p *TestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Initialize all tracking slices
	p.RefreshedResources = []string{}
	p.CreatedResources = []string{}
	p.DestroyedResources = []string{}
	p.UpdatedResources = []string{}
	p.ChangedCalls = []string{}
	
	// Register Container resource
	containerResource := &structs.Container{}
	containerProvider := &TestResourceProvider[*structs.Container]{plugin: p}
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"container",
		containerResource,
		containerProvider,
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
func (p *TestResourceProvider[T]) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	p.state = state
	p.functions = functions
	p.logger = logger
	return nil
}

// Create tracks the resource name for testing
func (p *TestResourceProvider[T]) Create(ctx context.Context, resource T) (T, error) {
	// Track the resource name when create is called
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.CreatedResources = append(p.plugin.CreatedResources, res.Metadata().ID)
		}
	}
	return resource, nil
}

// Destroy tracks the resource name for testing
func (p *TestResourceProvider[T]) Destroy(ctx context.Context, resource T, force bool) error {
	// Track the resource name when destroy is called
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.DestroyedResources = append(p.plugin.DestroyedResources, res.Metadata().ID)
		}
	}
	return nil
}

// Refresh tracks the resource name for testing
func (p *TestResourceProvider[T]) Refresh(ctx context.Context, resource T) error {
	// Track the resource name when refresh is called
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.RefreshedResources = append(p.plugin.RefreshedResources, res.Metadata().ID)
		}
	}
	return nil
}

// Update tracks the resource name for testing
func (p *TestResourceProvider[T]) Update(ctx context.Context, resource T) error {
	// Track the resource name when update is called
	if p.plugin != nil {
		// Use type assertion to check if resource implements the Resource interface
		if res, ok := any(resource).(types.Resource); ok && res != nil {
			p.plugin.UpdatedResources = append(p.plugin.UpdatedResources, res.Metadata().ID)
		}
	}
	return nil
}

// Changed tracks the comparison and always returns false for testing
func (p *TestResourceProvider[T]) Changed(ctx context.Context, old T, new T) (bool, error) {
	// Track the changed check when it's called
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

// Ensure EmbeddedTestPlugin implements Plugin interface
var _ plugins.Plugin = (*EmbeddedTestPlugin)(nil)

// Init initializes the embedded test plugin
func (p *EmbeddedTestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Register Container resource
	containerResource := &embedded.Container{}
	containerProvider := &TestResourceProvider[*embedded.Container]{}
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"container",
		containerResource,
		containerProvider,
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
	)
	if err != nil {
		return err
	}

	return nil
}