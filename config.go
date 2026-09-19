package xcl

import (
	"errors"
	"fmt"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// ErrEmptyConfiguration is returned by Apply when the configuration declares no
// blocks. Nothing is destroyed, created, changed or saved: use Destroy to
// remove everything. Check for it with errors.Is.
var ErrEmptyConfiguration = parser.ErrEmptyConfiguration

// Config defines the stack config
// It orchestrates high-level operations (Apply, Validate, Destroy)
// and manages the current state
type Config struct {
	currentState   *state.State             // Current state (private)
	pluginRegistry *registry.PluginRegistry // Config owns plugins
	stateStore     state.StateStore         // Persistence for state
	variables      map[string]any           // Variables for HCL parsing
	eventHandler   EventHandler             // Called for every lifecycle event during Apply and Destroy
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
// Resources that were applied before but are no longer in the configuration
// are destroyed first, children before parents, before anything is created or
// changed. Plugins are then executed for each resource in dependency order and
// the resulting state is saved. When a provider call fails, the progress made
// so far is saved before the error is returned, so the next apply resumes from
// it. A removed resource whose destroy fails stops the apply before anything
// is created or changed, and is kept as destroy_failed for the next apply to
// retry first.
// Nothing is saved when the configuration does not parse or validate, or when
// it declares no blocks, which returns ErrEmptyConfiguration: use Destroy to
// remove everything.
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

// Destroy removes every resource in the saved state, or in the in-memory state
// when no state store is configured. It needs no configuration: resources are
// destroyed children first, in the reverse of the order they were created in,
// using the parents each resource recorded when it was applied. Variables,
// outputs, modules, disabled blocks and registered types never reach a
// provider.
//
// The state is saved after each resource, so an interrupted destroy resumes
// from where it stopped. A resource that fails to be destroyed stays in the
// state as destroy_failed, together with everything it depends on, and is
// named in the returned error; calling Destroy again retries it. When nothing
// has been saved Destroy succeeds and writes nothing.
func (c *Config) Destroy() error {
	saved := c.currentState

	if c.stateStore != nil {
		if !c.stateStore.Exists() {
			return nil
		}

		loaded, err := c.stateStore.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		saved = loaded
	}

	if saved == nil || saved.ResourceCount() == 0 {
		return nil
	}

	// Create parser with StateStore, destroy saves through it after every resource
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		OnParserEvent:  parserEventHandler(c.eventHandler),
	})

	remaining, err := p.Destroy(saved)
	c.currentState = remaining

	return err
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
