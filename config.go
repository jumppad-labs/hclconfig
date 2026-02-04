package xcl

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/parser"
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
}

// NewConfig creates a new Config with functional options
// If no options are provided, creates a minimal config with no plugins or state
func NewConfig(opts ...ConfigOption) *Config {
	c := &Config{
		currentState: state.NewState(),
		variables:    map[string]any{},
	}

	// Apply all options
	for _, opt := range opts {
		opt(c)
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

// Validate parses config from the given paths and compares against existing state
// Returns a Diff showing what would change, without executing plugins
// Validates:
//   - HCL syntax and schema
//   - DAG has no cycles
//   - Resource dependencies are valid
func (c *Config) Validate(paths ...string) (*Diff, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}

	// Create parser with StateStore
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		Variables:      convertVariablesToStringMap(c.variables),
	})

	// Parse without executing plugins
	newState, err := p.Parse(false, paths...)
	if err != nil {
		return nil, err
	}

	// Build diff between current and new state
	diff := buildDiff(newState, c.currentState)
	return diff, nil
}

// Apply parses config from paths, loads existing state, and applies changes
// Automatically determines creates, updates, and destroys
// Executes plugins for each resource in dependency order
// Saves state after each successful resource operation for resumability
func (c *Config) Apply(paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	// Create parser with StateStore
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		Variables:      convertVariablesToStringMap(c.variables),
	})

	// Parser manages State independently (loads from store, parses, returns new state)
	newState, err := p.Parse(true, paths...)
	if err != nil {
		return err
	}

	// Adopt the new state
	c.currentState = newState

	// Save to store
	if c.stateStore != nil {
		if err := c.stateStore.Save(c.currentState); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
	}

	return nil
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
