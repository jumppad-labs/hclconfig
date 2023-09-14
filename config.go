package hclconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/hclconfig/types"
)

// Config defines the stack config
type Config struct {
	Resources []types.Resource `json:"resources"`
	contexts  map[types.Resource]*hcl.EvalContext
	bodies    map[types.Resource]*hclsyntax.Body
	sync      sync.Mutex
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
	c := &Config{
		Resources: []types.Resource{},
		contexts:  map[types.Resource]*hcl.EvalContext{},
		bodies:    map[types.Resource]*hclsyntax.Body{},
		sync:      sync.Mutex{},
	}

	return c
}

// FindResource returns the resource for the given name
// name is defined with the convention: resource.[type].[name]
// the keyword "resource" is a required component in the path to allow
// names of resources to contain "." and to enable easy separate of
// module parts.
//
// "module" is an optional path parameter: module.[module_name].resource.[type].[name]
// and is required when searching for resources that have the Module flag set.
//
// If a resource can not be found, resource will be null and an
// error will be returned
//
// e.g. to find a cluster named k3s
// r, err := c.FindResource("resource.cluster.k3s")
//
// e.g. to find a cluster named k3s in the module module1
// r, err := c.FindResource("module.module1.resource.cluster.k3s")
func (c *Config) FindResource(path string) (types.Resource, error) {
	fqdn, err := types.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	// this is an internal error and should not happen unless there is an issue with a provider
	// there was, hence why we are here
	if c.Resources == nil {
		return nil, fmt.Errorf("unable to find resources, reference to parent config does not exist. Ensure that the object has been added to the config: `config.ResourceInfo.AddChild(type)`")
	}

	for _, r := range c.Resources {
		if r.Metadata().Module == fqdn.Module &&
			r.Metadata().Type == fqdn.Type &&
			r.Metadata().Name == fqdn.Resource {
			return r, nil
		}
	}

	return nil, ResourceNotFoundError{path}
}

func (c *Config) FindRelativeResource(path string, parentModule string) (types.Resource, error) {
	fqdn, err := types.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	if parentModule != "" {
		mod := fmt.Sprintf("%s.%s", parentModule, fqdn.Module)

		// fqdn.Module could be nil
		mod = strings.Trim(mod, ".")
		fqdn.Module = mod
	}

	r, err := c.FindResource(fqdn.String())
	if err != nil {
		return nil, err
	}

	return r, nil
}

// FindResourcesByType returns the resources from the given type
func (c *Config) FindResourcesByType(t string) ([]types.Resource, error) {
	res := []types.Resource{}

	for _, r := range c.Resources {
		if r.Metadata().Type == t {
			res = append(res, r)
		}
	}

	if len(res) > 0 {
		return res, nil
	}

	return nil, ResourceNotFoundError{t}
}

// FindModuleResources returns an array of resources for the given module
// if includeSubModules is true then resources in any submodules
// are also returned
// if includeSubModules is false only the resources defined in the given module are returned
func (c *Config) FindModuleResources(module string, includeSubModules bool) ([]types.Resource, error) {
	fqdn, err := types.ParseFQRN(module)
	if err != nil {
		return nil, err
	}

	if fqdn.Type != types.TypeModule {
		return nil, fmt.Errorf("resource %s is not a module reference", module)
	}

	moduleString := fmt.Sprintf("%s.%s", fqdn.Module, fqdn.Resource)
	moduleString = strings.TrimPrefix(moduleString, ".")

	resources := []types.Resource{}

	for _, r := range c.Resources {
		match := false
		if includeSubModules && strings.HasPrefix(r.Metadata().Module, moduleString) {
			match = true
		}

		if !includeSubModules && r.Metadata().Module == moduleString {
			match = true
		}

		if match {
			resources = append(resources, r)
		}
	}

	if len(resources) > 0 {
		return resources, nil
	}

	return nil, ResourceNotFoundError{fqdn.Module}
}

// ResourceCount defines the number of resources in a config
func (c *Config) ResourceCount() int {
	return len(c.Resources)
}

// AppendResourcesFromConfig adds the resources in the given config to
// this config. If a resources all ready exists a ResourceExistsError
// error is returned
func (c *Config) AppendResourcesFromConfig(new *Config) error {
	c.sync.Lock()
	defer c.sync.Unlock()

	for _, r := range new.Resources {
		fqdn := types.FQDNFromResource(r).String()

		// does the resource already exist?
		if _, err := c.FindResource(fqdn); err == nil {
			return ResourceExistsError{Name: fqdn}
		}

		// we need to add the context and the body from the other resource
		// so we can use it when parsing
		c.addResource(r, new.contexts[r], new.bodies[r])
	}

	return nil
}

// AppendResource adds a given resource to the resource list
// if the resource already exists an error will be returned
func (c *Config) AppendResource(r types.Resource) error {
	c.sync.Lock()
	defer c.sync.Unlock()

	return c.addResource(r, nil, nil)
}

func (c *Config) addResource(r types.Resource, ctx *hcl.EvalContext, b *hclsyntax.Body) error {
	fqdn := types.FQDNFromResource(r)

	// set the ID
	r.Metadata().ID = fqdn.String()

	rf, err := c.FindResource(fqdn.String())
	if err == nil && rf != nil {
		return ResourceExistsError{r.Metadata().Name}
	}

	c.Resources = append(c.Resources, r)
	c.contexts[r] = ctx
	c.bodies[r] = b

	return nil
}

func (c *Config) RemoveResource(rf types.Resource) error {
	c.sync.Lock()
	defer c.sync.Unlock()

	pos := -1
	for i, r := range c.Resources {
		if rf.Metadata().Name == r.Metadata().Name &&
			rf.Metadata().Type == r.Metadata().Type &&
			rf.Metadata().Module == r.Metadata().Module {
			pos = i
			break
		}
	}

	// found the resource remove from the collection
	// preserve order
	if pos > -1 {
		c.Resources = append(c.Resources[:pos], c.Resources[pos+1:]...)

		// clean up the context and body
		delete(c.contexts, rf)
		delete(c.bodies, rf)
		return nil
	}

	return ResourceNotFoundError{}
}

// ToJSON converts the config to a serializable json string
// to unmarshal the output of this method back into a config you can use
// the Parser.UnmarshalJSON method
func (c *Config) ToJSON() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := json.NewEncoder(buf)

	enc.SetIndent("", " ")
	err := enc.Encode(c)
	if err != nil {
		return nil, fmt.Errorf("unable to encode config: %s", err)
	}

	return buf.Bytes(), nil
}

// ResourceDiff is a container for resources that have changed between
// two different configurations
type ResourceDiff struct {
	Added     []types.Resource
	Updated   []types.Resource
	Removed   []types.Resource
	Unchanged []types.Resource
}

// Diff compares the current configuration to the provided configuration and
// returns resources that have changed between the two configurations
func (c *Config) Diff(o *Config) (*ResourceDiff, error) {
	var new []types.Resource
	var changed []types.Resource
	var removed []types.Resource
	var unchanged []types.Resource

	for _, r := range o.Resources {
		// does the resource exist
		cr, err := c.FindResource(r.Metadata().ID)

		// check if the resource has been found
		if err != nil {
			// resource does not exist
			new = append(new, r)
			continue
		}

		// check if the resource has changed
		if cr.Metadata().Checksum != r.Metadata().Checksum {
			// resource has changes rebuild
			changed = append(changed, r)
			continue
		}
	}

	// check if there are resources in the state that are no longer
	// in the config
	for _, r := range c.Resources {
		found := false
		for _, r2 := range o.Resources {
			if r.Metadata().ID == r2.Metadata().ID {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, r)
		}
	}

	// now add any unchanged resources
	for _, r := range c.Resources {
		found := false
		for _, r2 := range new {
			if r.Metadata().ID == r2.Metadata().ID {
				found = true
				break
			}
		}

		for _, r2 := range changed {
			if r.Metadata().ID == r2.Metadata().ID {
				found = true
				break
			}
		}

		for _, r2 := range removed {
			if r.Metadata().ID == r2.Metadata().ID {
				found = true
				break
			}
		}

		if !found {
			unchanged = append(unchanged, r)
		}
	}

	return &ResourceDiff{
		Added:     new,
		Removed:   removed,
		Updated:   changed,
		Unchanged: unchanged,
	}, nil

}

func (c *Config) getContext(rf types.Resource) (*hcl.EvalContext, error) {
	if ctx, ok := c.contexts[rf]; ok {
		return ctx, nil
	}

	return nil, ResourceNotFoundError{}
}

func (c *Config) getBody(rf types.Resource) (*hclsyntax.Body, error) {
	if b, ok := c.bodies[rf]; ok {
		return b, nil
	}

	return nil, ResourceNotFoundError{}
}
