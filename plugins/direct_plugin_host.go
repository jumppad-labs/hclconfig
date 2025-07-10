package plugins

import (
	"context"
	"fmt"
)

// DirectPluginHost provides direct method calls to in-process plugins without gRPC overhead.
// This is used for testing and embedded plugins where the plugin runs in the same process.
type DirectPluginHost struct {
	plugin Plugin
	logger Logger
	state  State
}

// NewDirectPluginHost creates a new direct plugin host for in-process plugins
func NewDirectPluginHost(logger Logger, state State, plugin Plugin) (*DirectPluginHost, error) {
	// Initialize the plugin with logger and state
	if err := plugin.Init(logger, state); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}

	return &DirectPluginHost{
		plugin: plugin,
		logger: logger,
		state:  state,
	}, nil
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
func (h *DirectPluginHost) Create(entityType, entitySubType string, entityData []byte) error {
	return h.plugin.Create(entityType, entitySubType, entityData)
}

// Destroy deletes an existing entity
func (h *DirectPluginHost) Destroy(entityType, entitySubType string, entityData []byte) error {
	return h.plugin.Destroy(entityType, entitySubType, entityData)
}

// Refresh refreshes the plugin state
func (h *DirectPluginHost) Refresh(ctx context.Context) error {
	return h.plugin.Refresh(ctx)
}

// Update updates an existing entity
func (h *DirectPluginHost) Update(entityType, entitySubType string, entityData []byte) error {
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