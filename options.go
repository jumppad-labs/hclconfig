package xcl

import (
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// ConfigOption is a functional option for configuring Config
type ConfigOption func(*Config)

// WithPluginRegistry sets the plugin registry to use
// If not provided, only the builtin resource types will be available
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

// WithEventHandler sets a handler that is called for every lifecycle event
// during Apply and Destroy, such as a block being parsed or a provider's
// Create or Destroy starting or succeeding, and for the parse events during
// Validate
func WithEventHandler(handler EventHandler) ConfigOption {
	return func(c *Config) {
		c.eventHandler = handler
	}
}

// WithVariables sets variables to pass to HCL parsing
func WithVariables(vars map[string]any) ConfigOption {
	return func(c *Config) {
		c.variables = vars
	}
}
