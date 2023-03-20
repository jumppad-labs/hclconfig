package hclconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcl/hclsyntax"
	"github.com/shipyard-run/hclconfig/types"
)

// Config defines the stack config
type Config struct {
	Resources []types.Resource `json:"resources"`
	contexts  map[types.Resource]*hcl.EvalContext
	bodies    map[types.Resource]*hclsyntax.Body
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
func newConfig() *Config {
	c := &Config{
		Resources: []types.Resource{},
		contexts:  map[types.Resource]*hcl.EvalContext{},
		bodies:    map[types.Resource]*hclsyntax.Body{},
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
	fqdn, err := types.ParseFQDN(path)
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
	fqdn, err := types.ParseFQDN(path)
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

func (c *Config) FindRelativeModuleResources(module string, parent string, includeSubModules bool) ([]types.Resource, error) {
	fqdn, err := types.ParseFQDN(module)
	if err != nil {
		return nil, err
	}

	modulePath := module

	if parent != "" {
		modulePath = fmt.Sprintf("module.%s.%s", parent, fqdn.Module)
	}

	return c.FindModuleResources(modulePath, includeSubModules)
}

// FindModuleResources returns an array of resources for the given module
// if includeSubModules is true then all resources that may be included in a submodule
// are also returned
// if includeSubModules is false only the resources defined in the given module are returned
func (c *Config) FindModuleResources(module string, includeSubModules bool) ([]types.Resource, error) {
	fqdn, err := types.ParseFQDN(module)
	if err != nil {
		return nil, err
	}

	resources := []types.Resource{}

	for _, r := range c.Resources {
		match := false
		if includeSubModules && strings.HasPrefix(r.Metadata().Module, fqdn.Module) {
			match = true
		}

		if !includeSubModules && r.Metadata().Module == fqdn.Module {
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

// AddResource adds a given resource to the resource list
// if the resource already exists an error will be returned
func (c *Config) addResource(r types.Resource, ctx *hcl.EvalContext, b *hclsyntax.Body) error {
	rn := fmt.Sprintf("resource.%s.%s", r.Metadata().Type, r.Metadata().Name)
	if r.Metadata().Module != "" {
		rn = fmt.Sprintf("module.%s.resource.%s.%s", r.Metadata().Module, r.Metadata().Type, r.Metadata().Name)
	}

	rf, err := c.FindResource(rn)
	if err == nil && rf != nil {
		return ResourceExistsError{r.Metadata().Name}
	}

	c.Resources = append(c.Resources, r)
	c.contexts[r] = ctx
	c.bodies[r] = b

	r.Metadata().ParentConfig = c

	return nil
}

func (c *Config) removeResource(rf types.Resource) error {
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

		// clean up the context and body
		delete(c.contexts, rf)
		delete(c.bodies, rf)
		return nil
	}

	return ResourceNotFoundError{}
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
