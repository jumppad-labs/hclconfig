package state

import (
	"encoding/json"
	"fmt"
	"os"
)

type FileStateStore struct {
	path string
}

func NewFileStateStore(path string) (*FileStateStore, error) {
	// Check if the file exists
	// If not, create an empty state file
	// Else, load the state from the file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return createStateAtPath(path)
	}

	fss := &FileStateStore{path: path}
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

	fmt.Println(string(data))

	// Deserialize the state
	resources := []interface{}{}
	err = json.Unmarshal(data, &resources)
	if err != nil {
		return nil, fmt.Errorf("unable to deserialize state file at %s: %w", fs.path, err)
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

func createStateAtPath(path string) (*FileStateStore, error) {
	fs := &FileStateStore{path: path}
	s := NewState()
	err := fs.Save(s)
	if err != nil {
		return nil, fmt.Errorf("unable to create state file at %s: %w", path, err)
	}

	return fs, nil
}
