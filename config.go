package xcl

import (
	"errors"
	"fmt"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// Config defines the stack config
// It orchestrates high-level operations (Apply, Validate, Destroy)
// and manages the current state
type Config struct {
	currentState   *state.State             // Current state (private)
	pluginRegistry *registry.PluginRegistry // Config owns plugins
	stateStore     state.StateStore         // Persistence for state
	variables      map[string]any           // Variables for HCL parsing
	eventHandler   EventHandler             // Called for every lifecycle event during Apply
}

// NewConfig creates a new Config with functional options
// If no options are provided, creates a minimal config with only the builtin
// resource types and no state
func NewConfig(opts ...ConfigOption) *Config {
	c := &Config{
		currentState: state.NewState(),
		variables:    map[string]any{},
	}

	// Apply all options
	for _, opt := range opts {
		opt(c)
	}

	if c.pluginRegistry == nil {
		c.pluginRegistry = registry.NewPluginRegistry(logger.NewStdOutLogger())
	}

	return c
}

// GetResources returns all resources in current state
func (c *Config) GetResources() []any {
	if c.currentState == nil {
		return []any{}
	}
	return c.currentState.GetResources()
}

// FindResource finds a resource in current state by FQRN path
func (c *Config) FindResource(path string) (any, error) {
	if c.currentState == nil {
		return nil, state.ResourceNotFoundError{Resource: path}
	}
	return c.currentState.FindResource(path)
}

// ResourceCount returns number of resources in current state
func (c *Config) ResourceCount() int {
	if c.currentState == nil {
		return 0
	}
	return c.currentState.ResourceCount()
}

// Validate parses config from the given paths and reports whether it is valid,
// without executing plugins and without creating, changing or removing anything.
// A nil error means the configuration is valid; otherwise the returned error
// collects every problem found.
// Validates:
//   - HCL syntax and schema
//   - DAG has no cycles
//   - Resource dependencies are valid
func (c *Config) Validate(paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	// Create parser with StateStore
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		Variables:      convertVariablesToStringMap(c.variables),
		OnParserEvent:  parserEventHandler(c.eventHandler),
	})

	// Validate without resolving: no decode, no DAG walk, no plugins
	if err := p.Validate(paths...); err != nil {
		return err
	}

	return nil
}

// Apply parses config from paths, loads existing state, and applies changes.
// Executes plugins for each resource in dependency order and saves the
// resulting state. When a provider call fails, the progress made so far is
// saved before the error is returned, so the next apply resumes from it.
// Nothing is saved when the configuration does not parse or validate.
func (c *Config) Apply(paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	// Create parser with StateStore
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		Variables:      convertVariablesToStringMap(c.variables),
		OnParserEvent:  parserEventHandler(c.eventHandler),
	})

	// Parser manages State independently (loads from store, parses, returns new state)
	// A failed apply returns the progress it made along with the error
	newState, err := p.Apply(paths...)
	if newState == nil {
		return err
	}

	// Adopt the new state
	c.currentState = newState

	// Save to store
	if c.stateStore != nil {
		if saveErr := c.stateStore.Save(c.currentState); saveErr != nil {
			return errors.Join(err, fmt.Errorf("failed to save state: %w", saveErr))
		}
	}

	return err
}

// Destroy removes all resources currently in state
// Executes destroy in reverse dependency order
// Saves state after each successful destroy for resumability
func (c *Config) Destroy() error {
	// TODO: Implement destroy logic
	return nil
}

// convertVariablesToStringMap converts map[string]any to map[string]string
// This is needed for parser compatibility
func convertVariablesToStringMap(vars map[string]any) map[string]string {
	result := make(map[string]string)
	for k, v := range vars {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}
