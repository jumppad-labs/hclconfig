package state

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/types"
)

// State manages resource storage and querying
// This is the core resource registry used by Parser
type State struct {
	resources []any      // All resources in the configuration
	mu        sync.Mutex // Protects concurrent access
}

// NewState creates a new empty State
func NewState() *State {
	return &State{
		resources: []any{},
	}
}

// GetResources returns all resources
// This satisfies parser.ResourceProvider interface
func (s *State) GetResources() []any {
	return s.resources
}

// AppendResource adds a resource to the state
// If the given resource does not have ResourceBase embedded, an error is returned
func (s *State) AppendResource(r any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.addResource(r)
}

// RemoveResource removes a resource from the state
func (s *State) RemoveResource(rf any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos := -1
	rfMeta, err := types.GetMeta(rf)
	if err != nil {
		return ResourceNotFoundError{}
	}

	for i, r := range s.resources {
		rMeta, err := types.GetMeta(r)
		if err != nil {
			continue
		}
		if rfMeta.Name == rMeta.Name &&
			rfMeta.Type == rMeta.Type &&
			rfMeta.Module == rMeta.Module {
			pos = i
			break
		}
	}

	if pos > -1 {
		s.resources = append(s.resources[:pos], s.resources[pos+1:]...)
		return nil
	}

	return ResourceNotFoundError{}
}

// ResourceCount returns the number of resources
func (s *State) ResourceCount() int {
	return len(s.resources)
}

// FindResource returns the resource for the given FQRN path
// Thread-safe with mutex
func (s *State) FindResource(path string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.findResource(path)
}

// FindRelativeResource finds a resource relative to a parent module
func (s *State) FindRelativeResource(path string, parentModule string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fqdn, err := resources.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	if parentModule != "" {
		mod := fmt.Sprintf("%s.%s", parentModule, fqdn.Module)
		mod = strings.Trim(mod, ".")
		fqdn.Module = mod
	}

	return s.findResource(fqdn.String())
}

// FindResourcesByType returns all resources of the given type
func (s *State) FindResourcesByType(t string) ([]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := []any{}

	for _, r := range s.resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue
		}
		if meta.Type == t {
			res = append(res, r)
		}
	}

	if len(res) > 0 {
		return res, nil
	}

	return nil, ResourceNotFoundError{t}
}

// FindModuleResources returns resources in a module, optionally including submodules
func (s *State) FindModuleResources(module string, includeSubModules bool) ([]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fqdn, err := resources.ParseFQRN(module)
	if err != nil {
		return nil, err
	}

	if fqdn.Type != resources.TypeModule {
		return nil, fmt.Errorf("resource %s is not a module reference", module)
	}

	moduleString := fmt.Sprintf("%s.%s", fqdn.Module, fqdn.Resource)
	moduleString = strings.TrimPrefix(moduleString, ".")

	results := []any{}

	for _, r := range s.resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue
		}

		match := false
		if includeSubModules && strings.HasPrefix(meta.Module, moduleString) {
			match = true
		}

		if !includeSubModules && meta.Module == moduleString {
			match = true
		}

		if match {
			results = append(results, r)
		}
	}

	if len(results) > 0 {
		return results, nil
	}

	return nil, ResourceNotFoundError{fqdn.Module}
}

// Bytes returns the state serialized as bytes
func (s *State) Bytes() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return json.MarshalIndent(s.resources, "", "  ")
}

// addResource is the internal unlocked version
func (s *State) addResource(r any) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase embedded: %w", err)
	}

	fqdn := &resources.FQRN{
		Module:   meta.Module,
		Resource: meta.Name,
		Type:     meta.Type,
	}

	// Set the ID
	meta.ID = fqdn.String()

	// Check if already exists
	rf, findErr := s.findResource(fqdn.String())
	if findErr == nil && rf != nil {
		return ResourceExistsError{meta.Name}
	}

	s.resources = append(s.resources, r)
	return nil
}

// findResource is the internal unlocked version
func (s *State) findResource(path string) (any, error) {
	fqdn, err := resources.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	for _, r := range s.resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue
		}
		if meta.Module == fqdn.Module &&
			meta.Type == fqdn.Type &&
			meta.Name == fqdn.Resource {
			return r, nil
		}
	}

	return nil, ResourceNotFoundError{fqdn.StringWithoutAttribute()}
}
