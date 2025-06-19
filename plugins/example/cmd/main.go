package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/plugins/example"
)

// main function for the plugin binary
func main() {
	// Create the plugin implementation
	personPlugin := &example.PersonPlugin{}

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
