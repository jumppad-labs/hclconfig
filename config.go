package hclconfig

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/hclconfig/errors"
	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/silas/dag"
)

// Config defines the stack config
type Config struct {
	Resources []any `json:"resources"`
	contexts  map[any]*hcl.EvalContext
	bodies    map[any]*hclsyntax.Body
	sync      sync.Mutex
}

// ResourceNotFoundError is thrown when a resource could not be found
type ResourceNotFoundError struct {
	Name string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource not found: %s", e.Name)
}

// ResourceExistsError is thrown when a resource already exists in the resource list
type ResourceExistsError struct {
	Name string
}

func (e ResourceExistsError) Error() string {
	return fmt.Sprintf("resource already exists: %s", e.Name)
}

// New creates a new Config
func NewConfig() *Config {
	c := &Config{
		Resources: []any{},
		contexts:  map[any]*hcl.EvalContext{},
		bodies:    map[any]*hclsyntax.Body{},
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
func (c *Config) FindResource(path string) (any, error) {
	c.sync.Lock()
	defer c.sync.Unlock()

	return c.findResource(path)
}

// local version of FindResource that does not lock the config
func (c *Config) findResource(path string) (any, error) {
	fqdn, err := resources.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	// this is an internal error and should not happen unless there is an issue with a provider
	// there was, hence why we are here
	if c.Resources == nil {
		return nil, fmt.Errorf("unable to find resources, reference to parent config does not exist. Ensure that the object has been added to the config: `config.Info.AddChild(type)`")
	}

	for _, r := range c.Resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		if meta.Module == fqdn.Module &&
			meta.Type == fqdn.Type &&
			meta.Name == fqdn.Resource {
			return r, nil
		}
	}

	return nil, ResourceNotFoundError{fqdn.StringWithoutAttribute()}
}

func (c *Config) FindRelativeResource(path string, parentModule string) (any, error) {
	c.sync.Lock()
	defer c.sync.Unlock()

	fqdn, err := resources.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	if parentModule != "" {
		mod := fmt.Sprintf("%s.%s", parentModule, fqdn.Module)

		// fqdn.Module could be nil
		mod = strings.Trim(mod, ".")
		fqdn.Module = mod
	}

	r, err := c.findResource(fqdn.String())
	if err != nil {
		return nil, err
	}

	return r, nil
}

// FindResourcesByType returns the resources from the given type
func (c *Config) FindResourcesByType(t string) ([]any, error) {
	c.sync.Lock()
	defer c.sync.Unlock()

	res := []any{}

	for _, r := range c.Resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
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

// FindModuleResources returns an array of resources for the given module
// if includeSubModules is true then resources in any submodules
// are also returned
// if includeSubModules is false only the resources defined in the given module are returned
func (c *Config) FindModuleResources(module string, includeSubModules bool) ([]any, error) {
	c.sync.Lock()
	defer c.sync.Unlock()

	fqdn, err := resources.ParseFQRN(module)
	if err != nil {
		return nil, err
	}

	if fqdn.Type != resources.TypeModule {
		return nil, fmt.Errorf("resource %s is not a module reference", module)
	}

	moduleString := fmt.Sprintf("%s.%s", fqdn.Module, fqdn.Resource)
	moduleString = strings.TrimPrefix(moduleString, ".")

	resources := []any{}

	for _, r := range c.Resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		match := false
		if includeSubModules && strings.HasPrefix(meta.Module, moduleString) {
			match = true
		}

		if !includeSubModules && meta.Module == moduleString {
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
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		fqdn := &resources.FQRN{Module: meta.Module, Resource: meta.Name, Type: meta.Type}
		fqdnString := fqdn.String()

		// does the resource already exist?
		if _, err := c.findResource(fqdnString); err == nil {
			return ResourceExistsError{Name: fqdnString}
		}

		// we need to add the context and the body from the other resource
		// so we can use it when parsing
		c.addResource(r, new.contexts[r], new.bodies[r])
	}

	return nil
}

// AppendResource adds a given resource to the resource list
// if the resource already exists an error will be returned
func (c *Config) AppendResource(r any) error {
	c.sync.Lock()
	defer c.sync.Unlock()

	return c.addResource(r, nil, nil)
}

func (c *Config) RemoveResource(rf any) error {
	c.sync.Lock()
	defer c.sync.Unlock()

	pos := -1
	rfMeta, err := types.GetMeta(rf)
	if err != nil {
		return ResourceNotFoundError{}
	}
	for i, r := range c.Resources {
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

// WalkCallback is called with the resource when the graph processes that particular node
type WalkCallback func(r any) error

// Walk creates a Directed Acyclic Graph for the configuration resources depending on their
// links and references. All the resources defined in the graph are traversed and
// the provided callback is executed for every resource in the graph.
//
// Any error returned from the ProcessCallback function halts execution of any other
// callback for resources in the graph.
//
// Specifying the reverse option to 'true' causes the graph to be traversed in reverse
// order.
func (c *Config) Walk(wf WalkCallback, reverse bool) error {
	// We need to ensure that Process does not execute the callback when
	// any other callback returns an error.
	// Unfortunately returning an error with tfdiags does not stop the walk
	hasError := atomic.Bool{}

	pe := errors.NewConfigError()

	errs := c.walk(
		func(v dag.Vertex) (diags dag.Diagnostics) {

			// v should be a resource (either builtin or schema-generated)
			r := v

			// if this is the root module or is disabled skip
			meta, err := types.GetMeta(r)
			if err != nil {
				return nil // Skip resources without ResourceBase
			}
			disabled, err := types.GetDisabled(r)
			if err != nil {
				disabled = false
			}
			if (meta.Type == resources.TypeRoot || meta.Type == resources.TypeModule) || disabled {
				return nil
			}

			// call the callback only if a previous error has not occurred
			if hasError.Load() {
				return nil
			}

			err = wf(r)
			if err != nil {
				// set the global error mutex to stop further processing
				hasError.Store(true)

				return diags.Append(err)
			}

			return nil
		},
		reverse,
	)

	for _, e := range errs {
		pe.AppendError(e)
	}

	if len(pe.Errors) > 0 {
		return pe
	}

	return nil
}

// Until parse is called the HCL configuration is not deserialized into
// the structs. We have to do this using a graph as some inputs depend on
// outputs from other resources, therefore we need to process this is strict order
func (c *Config) walk(wf dag.WalkFunc, reverse bool) []error {
	// build the graph
	d, err := doYaLikeDAGs(c)
	if err != nil {
		return []error{err}
	}

	// reduce the graph nodes to unique instances
	d.TransitiveReduction()

	// validate the dependency graph is ok
	err = d.Validate()
	if err != nil {
		return []error{fmt.Errorf("unable to validate dependency graph: %w", err)}
	}

	// define the walker callback that will be called for every node in the graph
	w := dag.Walker{}
	w.Callback = wf
	w.Reverse = reverse

	// update the dag and process the nodes
	log.SetOutput(io.Discard)

	errs := []error{}
	w.Update(d)
	diags := w.Wait()
	if diags.HasErrors() {
		errs = append(errs, diags.Err().(errwrap.Wrapper).WrappedErrors()...)

		return errs
	}

	return nil
}

func (c *Config) addResource(r any, ctx *hcl.EvalContext, b *hclsyntax.Body) error {
	// Get metadata using helper function
	meta, err := types.GetMeta(r)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase embedded: %w", err)
	}

	// Create FQRN from metadata
	fqdn := &resources.FQRN{
		Module:   meta.Module,
		Resource: meta.Name,
		Type:     meta.Type,
	}

	// set the ID
	meta, err = types.GetMeta(r)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase embedded: %w", err)
	}
	meta.ID = fqdn.String()

	rf, findErr := c.findResource(fqdn.String())
	if findErr == nil && rf != nil {
		for _, res := range c.Resources {
			resMeta, err := types.GetMeta(res)
			if err == nil {
				fmt.Println("Resource already exists:", resMeta.ID)
			}
		}
		return ResourceExistsError{meta.Name}
	}

	// Now we can store any type of resource (builtin or schema-generated)
	c.Resources = append(c.Resources, r)
	c.contexts[r] = ctx
	c.bodies[r] = b

	return nil
}

func (c *Config) getContext(rf any) (*hcl.EvalContext, error) {
	if ctx, ok := c.contexts[rf]; ok {
		return ctx, nil
	}

	return nil, ResourceNotFoundError{}
}

func (c *Config) getBody(rf any) (*hclsyntax.Body, error) {
	if b, ok := c.bodies[rf]; ok {
		return b, nil
	}

	return nil, ResourceNotFoundError{}
}

func NewQuerier[T any](c *Config) *Querier[T] {
	return &Querier[T]{config: c}
}

type Querier[T any] struct {
	config *Config
}

func (q *Querier[T]) FindResource(path string) (T, error) {
	for _, r := range q.config.Resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		if meta.ID == path {
			return r.(T), nil
		}
	}

	// return a zero value of T and an error
	var result T
	return result, ResourceNotFoundError{path}
}

func (q *Querier[T]) FindResourcesByType() ([]T, error) {
	return nil, fmt.Errorf("not implemented")
}
