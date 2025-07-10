package hclconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStateStore implements StateStore using file-based persistence.
// State is stored as JSON in a file within the state directory.
type FileStateStore struct {
	stateDir  string
	stateFile string
	registry  *PluginRegistry
	mu        sync.Mutex
}

// NewFileStateStore creates a new file-based state store.
// If stateDir is empty, it defaults to ".hclconfig/state" in the current directory.
func NewFileStateStore(stateDir string, registry *PluginRegistry) *FileStateStore {
	if stateDir == "" {
		stateDir = filepath.Join(".", ".hclconfig", "state")
	}

	return &FileStateStore{
		stateDir:  stateDir,
		stateFile: filepath.Join(stateDir, "state.json"),
		registry:  registry,
	}
}

// Load retrieves the previously saved configuration state from the file.
func (f *FileStateStore) Load() (*Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Deserialize the JSON to Config
	return f.unmarshalConfig(data)
}

// Save persists the current configuration state to the file.
func (f *FileStateStore) Save(config *Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Serialize the Config to JSON
	data, err := f.marshalConfig(config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// Ensure the state directory exists
	if err := os.MkdirAll(f.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to a temporary file first for atomic writes
	tmpFile := f.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	// Atomically rename the temporary file to the actual state file
	if err := os.Rename(tmpFile, f.stateFile); err != nil {
		// Clean up the temporary file if rename fails
		os.Remove(tmpFile)
		return fmt.Errorf("failed to save state file: %w", err)
	}

	return nil
}

// Exists returns true if a saved state file exists.
func (f *FileStateStore) Exists() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, err := os.Stat(f.stateFile)
	return err == nil
}

// Clear removes the saved state file.
func (f *FileStateStore) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := os.Remove(f.stateFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear state: %w", err)
	}

	return nil
}

// marshalConfig serializes a Config to JSON
func (f *FileStateStore) marshalConfig(config *Config) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := json.NewEncoder(buf)
	enc.SetIndent("", " ")
	
	err := enc.Encode(config)
	if err != nil {
		return nil, fmt.Errorf("unable to encode config: %w", err)
	}

	return buf.Bytes(), nil
}

// unmarshalConfig deserializes JSON to a Config with proper resource types
func (f *FileStateStore) unmarshalConfig(data []byte) (*Config, error) {
	conf := NewConfig()

	var objMap map[string]*json.RawMessage
	err := json.Unmarshal(data, &objMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Handle the case where there are no resources
	if objMap["resources"] == nil {
		return conf, nil
	}

	var rawMessagesForResources []*json.RawMessage
	err = json.Unmarshal(*objMap["resources"], &rawMessagesForResources)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources: %w", err)
	}

	for _, m := range rawMessagesForResources {
		mm := map[string]any{}
		err := json.Unmarshal(*m, &mm)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		meta := mm["meta"].(map[string]any)

		// Create resource using the registry
		r, err := f.registry.CreateResource(meta["type"].(string), meta["name"].(string))
		if err != nil {
			return nil, fmt.Errorf("failed to create resource %s.%s: %w", meta["type"], meta["name"], err)
		}

		// Unmarshal the resource data into the concrete type
		resData, _ := json.Marshal(mm)
		if err := json.Unmarshal(resData, r); err != nil {
			return nil, fmt.Errorf("failed to populate resource %s.%s: %w", meta["type"], meta["name"], err)
		}

		conf.addResource(r, nil, nil)
	}

	return conf, nil
}