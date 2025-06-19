package main

import (
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/plugins/example/entities"
)

var _ plugins.Plugin = &ExamplePlugin{}

type ExamplePlugin struct {
	plugins.PluginBase
}

// Init is called by the HCLConfig framework to initialize the plugin.
func (p *ExamplePlugin) Init() error {
	// Register the types that this plugin can handle
	err := p.RegisterType(
		"example",
		"person",
		&entities.Person{},
		&entities.Provider{},
	)
	if err != nil {
		p.Log().Error("Failed to register type", "error", err)
		return err
	}

	// Log is a plugin method that passes the log message back to the main process
	// logging can not be done in the plugin itself as there is no stdout/stderr
	p.Log().Info("ExamplePlugin initialized")

	return nil
}
