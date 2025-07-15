package hclconfig

import (
	"context"

	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/hclconfig/types"
)

// TestPluginConfig represents the configuration for the test plugin
type TestPluginConfig struct {
	// Empty config for testing
}

// TestPlugin provides test resource types for testing the parser
type TestPlugin struct {
	plugins.PluginBase
}

// GetConfigType is now automatically provided by PluginBase

// Ensure TestPlugin implements Plugin interface
var _ plugins.Plugin = (*TestPlugin)(nil)

// Init initializes the test plugin with test resource types
func (p *TestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Register Container resource
	containerResource := &structs.Container{}
	containerProvider := &TestResourceProvider[*structs.Container]{}
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
	sidecarProvider := &TestResourceProvider[*structs.Sidecar]{}
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
	networkProvider := &TestResourceProvider[*structs.Network]{}
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
	templateProvider := &TestResourceProvider[*structs.Template]{}
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
	logger    logger.Logger
	state     plugins.State
	functions plugins.ProviderFunctions
}

// Init initializes the test provider
func (p *TestResourceProvider[T]) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger, config TestPluginConfig) error {
	p.state = state
	p.functions = functions
	p.logger = logger
	return nil
}

// Create is a no-op for testing
func (p *TestResourceProvider[T]) Create(ctx context.Context, resource T) (T, error) {
	return resource, nil
}

// Destroy is a no-op for testing
func (p *TestResourceProvider[T]) Destroy(ctx context.Context, resource T, force bool) error {
	return nil
}

// Refresh is a no-op for testing
func (p *TestResourceProvider[T]) Refresh(ctx context.Context, resource T) error {
	return nil
}

// Update is a no-op for testing
func (p *TestResourceProvider[T]) Update(ctx context.Context, resource T) error {
	return nil
}

// Changed always returns false for testing
func (p *TestResourceProvider[T]) Changed(ctx context.Context, old T, new T) (bool, error) {
	return false, nil
}

// Functions returns no functions
func (p *TestResourceProvider[T]) Functions() plugins.ProviderFunctions {
	return p.functions
}
