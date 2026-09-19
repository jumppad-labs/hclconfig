package plugins

import "context"

// PluginHost is the unified interface for both in-process and external plugins.
// This interface abstracts the communication mechanism, allowing the rest of the
// codebase to work with plugins regardless of whether they are in-process or external.
type PluginHost interface {
	// GetTypes returns the types handled by the plugin
	GetTypes() []RegisteredType

	// Validate validates the given entity data
	Validate(entityType, entitySubType string, entityData []byte) error

	// Create creates a new entity
	Create(entityType, entitySubType string, entityData []byte) ([]byte, error)

	// Destroy deletes an existing entity
	Destroy(entityType, entitySubType string, entityData []byte) error

	// Read reports the real entity, given its saved and configured copies
	Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error)

	// Update updates an existing entity
	Update(entityType, entitySubType string, entityData []byte) ([]byte, error)

	// Changed checks if the entity has changed by comparing old and new
	Changed(entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error)

	// Stop shuts down the plugin host and cleans up resources
	Stop()
}
