package example

import (
	"github.com/jumppad-labs/hclconfig/plugins"
)

// PersonPlugin demonstrates how to create a complete plugin
// that implements the Plugin interface
type PersonPlugin struct {
	plugins.PluginBase
}

// Ensure PersonPlugin implements the Plugin interface
var _ plugins.Plugin = (*PersonPlugin)(nil)

// Init is called by the HCLConfig framework to initialize the plugin.
// This is where you register all the resource types your plugin handles.
func (p *PersonPlugin) Init(logger plugins.Logger, state plugins.State) error {
	// Create instances of resources and providers
	personResource := &Person{}
	personProvider := &ExampleProvider{}

	// Register the Person resource type
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",     // Top-level type (usually "resource")
		"person",       // Sub-type (your specific resource type)
		personResource, // Instance of the resource struct
		personProvider, // Instance of the provider
	)

	if err != nil {
		return err
	}

	return nil
}
