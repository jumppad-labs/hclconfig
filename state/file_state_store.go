package state

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jumppad-labs/xcl/plugins/registry"
)

type FileStateStore struct {
	path     string
	registry *registry.PluginRegistry
}

func NewFileStateStore(path string, registry *registry.PluginRegistry) (*FileStateStore, error) {
	// Check if the file exists
	// If not, create an empty state file
	// Else, load the state from the file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return createStateAtPath(path, registry)
	}

	fss := &FileStateStore{
		path:     path,
		registry: registry,
	}
	return fss, nil
}

// Load the previously saved configuration state from the file
func (fs *FileStateStore) Load() (*State, error) {
	if _, err := os.Stat(fs.path); err != nil {
		return nil, fmt.Errorf("state file does not exist at %s", fs.path)
	}

	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, fmt.Errorf("unable to read state file at %s: %w", fs.path, err)
	}

	// Phase 1: Unmarshal to raw messages to preserve JSON structure
	var rawMessages []*json.RawMessage
	err = json.Unmarshal(data, &rawMessages)
	if err != nil {
		return nil, fmt.Errorf("unable to deserialize state file at %s: %w", fs.path, err)
	}

	// Phase 2: Create typed resources and unmarshal into them
	resources := []any{}
	for _, rawMsg := range rawMessages {
		// Peek at the metadata to get type and name
		var metadata map[string]any
		err := json.Unmarshal(*rawMsg, &metadata)
		if err != nil {
			// Skip malformed entries
			continue
		}

		// Extract meta information
		metaMap, ok := metadata["meta"].(map[string]any)
		if !ok {
			// Skip resources without proper metadata
			continue
		}

		resourceType, ok := metaMap["type"].(string)
		if !ok {
			// Skip resources without type
			continue
		}

		resourceName, ok := metaMap["name"].(string)
		if !ok {
			// Skip resources without name
			continue
		}

		// Create a typed resource reference using the registry
		typedResource, err := fs.registry.CreateResource(resourceType, resourceName)
		if err != nil {
			// Skip unknown resource types (plugin not loaded)
			continue
		}

		// Re-marshal and unmarshal into the typed reference
		resData, err := json.Marshal(metadata)
		if err != nil {
			// Skip if we can't re-marshal
			continue
		}

		err = json.Unmarshal(resData, typedResource)
		if err != nil {
			// Skip if unmarshal into typed resource fails
			continue
		}

		// Append the typed resource pointer
		resources = append(resources, typedResource)
	}

	s := NewState()
	s.resources = resources

	return s, nil
}

// Exists checks if the state file exists
func (fs *FileStateStore) Exists() bool {
	if _, err := os.Stat(fs.path); err == nil {
		return true
	}

	return false
}

// Clear removes the state file
func (fs *FileStateStore) Clear() error {
	return os.Remove(fs.path)
}

// Save the current configuration state to the file
func (fs *FileStateStore) Save(state *State) error {
	d, err := state.Bytes()
	if err != nil {
		return fmt.Errorf("unable to serialize state: %w", err)
	}

	// Remove the existing file if it exists
	if _, err := os.Stat(fs.path); err == nil {
		err = os.Remove(fs.path)
		if err != nil {
			return fmt.Errorf("unable to remove existing state file: %w", err)
		}
	}

	err = os.WriteFile(fs.path, d, 0644)
	if err != nil {
		return fmt.Errorf("unable to write state file: %w", err)
	}

	return nil
}

func createStateAtPath(path string, registry *registry.PluginRegistry) (*FileStateStore, error) {
	fs := &FileStateStore{
		path:     path,
		registry: registry,
	}
	s := NewState()
	err := fs.Save(s)
	if err != nil {
		return nil, fmt.Errorf("unable to create state file at %s: %w", path, err)
	}

	return fs, nil
}
