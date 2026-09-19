package plugins

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/logger"
)

// DirectPluginHost provides direct method calls to in-process plugins without gRPC overhead.
// This is used for testing and embedded plugins where the plugin runs in the same process.
type DirectPluginHost struct {
	plugin Plugin
	logger Logger
	state  State
}

// NewDirectPluginHost creates a new direct plugin host for in-process plugins.
// The plugin logs to log with every message tagged plugin=<Go type name of
// the plugin>, so its logs can be told apart from the host's.
func NewDirectPluginHost(log Logger, state State, plugin Plugin) (*DirectPluginHost, error) {
	log = logger.WithTag(log, "plugin", pluginTypeName(plugin))

	// Initialize the plugin with logger and state
	if err := plugin.Init(log, state); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}

	if log != nil {
		log.Debug("plugin loaded", "block_types", resourceTypeNames(plugin.GetTypes()))
	}

	return &DirectPluginHost{
		plugin: plugin,
		logger: log,
		state:  state,
	}, nil
}

// resourceTypeNames returns the comma separated block types of the resource
// types in registered, i.e. "postgres, app"
func resourceTypeNames(registered []RegisteredType) string {
	names := []string{}
	for _, t := range registered {
		if t.Type == "resource" {
			names = append(names, t.SubType)
		}
	}

	return strings.Join(names, ", ")
}

// pluginTypeName returns the name of the plugin's Go type, i.e. ExamplePlugin
// for an *ExamplePlugin
func pluginTypeName(plugin Plugin) string {
	t := reflect.TypeOf(plugin)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t.Name()
}

// GetTypes returns the types handled by the plugin
func (h *DirectPluginHost) GetTypes() []RegisteredType {
	return h.plugin.GetTypes()
}

// Validate validates the given entity data
func (h *DirectPluginHost) Validate(entityType, entitySubType string, entityData []byte) error {
	return h.plugin.Validate(entityType, entitySubType, entityData)
}

// Create creates a new entity
func (h *DirectPluginHost) Create(entityType, entitySubType string, entityData []byte) ([]byte, error) {
	return h.plugin.Create(entityType, entitySubType, entityData)
}

// Destroy deletes an existing entity
func (h *DirectPluginHost) Destroy(entityType, entitySubType string, entityData []byte) error {
	return h.plugin.Destroy(entityType, entitySubType, entityData)
}

// Read reports the real entity, given its saved and configured copies
func (h *DirectPluginHost) Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	return h.plugin.Read(ctx, entityType, entitySubType, oldEntityData, newEntityData)
}

// Update updates an existing entity
func (h *DirectPluginHost) Update(entityType, entitySubType string, entityData []byte) ([]byte, error) {
	return h.plugin.Update(entityType, entitySubType, entityData)
}

// Changed checks if the entity has changed by comparing old and new
func (h *DirectPluginHost) Changed(entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error) {
	return h.plugin.Changed(entityType, entitySubType, oldEntityData, newEntityData)
}

// Stop is a no-op for direct plugins as there's nothing to clean up
func (h *DirectPluginHost) Stop() {
	// No cleanup needed for in-process plugins
}

// Ensure DirectPluginHost implements PluginHost interface
var _ PluginHost = (*DirectPluginHost)(nil)
