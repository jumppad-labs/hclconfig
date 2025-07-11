package hclconfig

import (
	"testing"

	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins/example/pkg/person"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/stretchr/testify/require"
)

// TestPluginRegistration tests that we can register and use plugins
func TestPluginRegistration(t *testing.T) {
	// Create a new parser
	parser := NewParser(nil)

	// Create a simple test plugin
	plugin := &SimpleTestPlugin{}

	// Register the plugin
	err := parser.RegisterPlugin(plugin)
	require.NoError(t, err, "Should register plugin without error")

	// Verify the plugin was added to the registry
	require.Len(t, parser.pluginRegistry.GetPluginHosts(), 1, "Should have one plugin host")

	// Try to create a resource using the plugin registry
	resource, err := parser.pluginRegistry.CreateResource("person", "test_person")
	require.NoError(t, err, "Should create resource from plugin")
	require.NotNil(t, resource, "Resource should not be nil")
	require.Equal(t, "test_person", resource.Metadata().Name)
	require.Equal(t, "person", resource.Metadata().Type)
}

// TestPluginResourceCreationWithFallback tests plugin creation with fallback to registered types
func TestPluginResourceCreationWithFallback(t *testing.T) {
	parser := NewParser(nil)

	// Try to create a resource that doesn't exist in plugins (should fall back to registered types)
	// This should fail since we don't have any registered types for "nonexistent"
	_, err := parser.pluginRegistry.CreateResource("nonexistent", "test")
	require.Error(t, err, "Should fail to create nonexistent resource type")
	require.Contains(t, err.Error(), "not found in any registered plugin")
}

// SimpleTestPlugin is a simple test plugin for testing
type SimpleTestPlugin struct {
	plugins.PluginBase
}

// Init initializes the test plugin
func (p *SimpleTestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Create test person resource and provider
	personResource := &person.Person{}
	personProvider := &person.ExampleProvider{}

	// Register the Person resource type with the plugin
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",     // Top-level type
		"person",       // Sub-type
		personResource, // Resource instance
		personProvider, // Provider instance
	)
}