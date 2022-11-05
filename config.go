package hclconfig

import (
	"fmt"
	"strings"

	"github.com/shipyard-run/hclconfig/types"
)

// Config defines the stack config
type Config struct {
	Resources []types.Resource `json:"resources"`
}

func IsRegisteredType(t types.ResourceType) bool {
	return true
}

// ResourceNotFoundError is thrown when a resource could not be found
type ResourceNotFoundError struct {
	Name string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("Resource not found: %s", e.Name)
}

// ResourceExistsError is thrown when a resource already exists in the resource list
type ResourceExistsError struct {
	Name string
}

func (e ResourceExistsError) Error() string {
	return fmt.Sprintf("Resource already exists: %s", e.Name)
}

// New creates a new Config
func NewConfig() *Config {
	c := &Config{}

	return c
}

// FindModuleResources returns an array of resources for the given module
func (c *Config) FindModuleResources(name string) ([]types.Resource, error) {
	resources := []types.Resource{}

	parts := strings.Split(name, ".")

	for _, r := range c.Resources {
		if r.Info().Module == parts[1] {
			resources = append(resources, r)
		}
	}

	if len(resources) > 0 {
		return resources, nil
	}

	return nil, ResourceNotFoundError{name}
}

// FindResource returns the resource for the given name
// name is defined with the convention [module].[type].[name]
// if a resource can not be found resource will be null and an
// error will be returned
//
// e.g. to find a cluster named k3s
// r, err := c.FindResource("cluster.k3s")
//
// simple.consul.container.consul
func (c *Config) FindResource(name string) (types.Resource, error) {
	parts := strings.Split(name, ".")

	typeIndex := 0
	if len(parts) > 2 {
		typeIndex = len(parts) - 1
	}

	module := ""
	if typeIndex > 0 {
		module = strings.Join(parts[:typeIndex-1], ".")
	}

	// the name could contain . so join after the first
	typ := parts[typeIndex]
	n := strings.Join(parts[typeIndex+1:], ".")

	// this is an internal error and should not happen unless there is an issue with a provider
	// there was, hence why we are here
	if c.Resources == nil {
		return nil, fmt.Errorf("unable to find resources, reference to parent config does not exist. Ensure that the object has been added to the config: `config.ResourceInfo.AddChild(type)`")
	}

	for _, r := range c.Resources {
		if r.Info().Module == module &&
			r.Info().Type == types.ResourceType(typ) &&
			r.Info().Name == n {
			return r, nil
		}
	}

	return nil, ResourceNotFoundError{name}
}

// FindResourcesByType returns the resources from the given type
func (c *Config) FindResourcesByType(t string) []types.Resource {
	res := []types.Resource{}

	for _, r := range c.Resources {
		if r.Info().Type == types.ResourceType(t) {
			res = append(res, r)
		}
	}

	return res
}

// AddResource adds a given resource to the resource list
// if the resource already exists an error will be returned
func (c *Config) AddResource(r types.Resource) error {
	rn := fmt.Sprintf("%s.%s", r.Info().Name, r.Info().Type)
	if r.Info().Module != "" {
		rn = fmt.Sprintf("%s.%s.%s", r.Info().Module, r.Info().Type, r.Info().Name)
	}

	rf, err := c.FindResource(rn)
	if err == nil && rf != nil {
		return ResourceExistsError{r.Info().Name}
	}

	c.Resources = append(c.Resources, r)

	return nil
}

func (c *Config) RemoveResource(rf types.Resource) error {
	pos := -1
	for i, r := range c.Resources {
		if rf == r {
			pos = i
			break
		}
	}

	// found the resource remove from the collection
	// preserve order
	if pos > -1 {
		c.Resources = append(c.Resources[:pos], c.Resources[pos+1:]...)
		return nil
	}

	return ResourceNotFoundError{}
}

// ResourceCount defines the number of resources in a config
func (c *Config) ResourceCount() int {
	return len(c.Resources)
}
