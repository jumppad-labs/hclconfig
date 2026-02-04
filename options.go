package xcl

import (
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// ConfigOption is a functional option for configuring Config
type ConfigOption func(*Config)

// WithPluginRegistry sets the plugin registry to use
// If not provided, no plugins will be available
func WithPluginRegistry(pr *registry.PluginRegistry) ConfigOption {
	return func(c *Config) {
		c.pluginRegistry = pr
	}
}

// WithStateStore sets the state store for persistence
// Uses the existing state.StateStore interface from state/state_store.go
// If not provided, state will not be persisted
func WithStateStore(ss state.StateStore) ConfigOption {
	return func(c *Config) {
		c.stateStore = ss
	}
}

// WithVariables sets variables to pass to HCL parsing
func WithVariables(vars map[string]any) ConfigOption {
	return func(c *Config) {
		c.variables = vars
	}
}
