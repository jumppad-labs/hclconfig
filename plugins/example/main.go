package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/plugins/example/pkg/person"
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
	personResource := &person.Person{}
	personProvider := &person.ExampleProvider{}

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

// main function for the plugin binary
func main() {
	// Create the plugin implementation
	personPlugin := &PersonPlugin{}

	// Serve the plugin using go-plugin
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"plugin": &plugins.GRPCPlugin{
				Impl: personPlugin,
			},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
