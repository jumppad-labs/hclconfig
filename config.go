package hclconfig

import (
	"bytes"
	"encoding/json"
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
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/silas/dag"
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
	c.sync.Lock()
	defer c.sync.Unlock()

	return c.findResource(path)
}

// local version of FindResource that does not lock the config
func (c *Config) findResource(path string) (types.Resource, error) {
	fqdn, err := types.ParseFQRN(path)
	if err != nil {
		return nil, err
	}

	// this is an internal error and should not happen unless there is an issue with a provider
	// there was, hence why we are here
	if c.Resources == nil {
		return nil, fmt.Errorf("unable to find resources, reference to parent config does not exist. Ensure that the object has been added to the config: `config.Info.AddChild(type)`")
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
	c.sync.Lock()
	defer c.sync.Unlock()

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

	r, err := c.findResource(fqdn.String())
	if err != nil {
		return nil, err
	}

	return r, nil
}

// FindResourcesByType returns the resources from the given type
func (c *Config) FindResourcesByType(t string) ([]types.Resource, error) {
	c.sync.Lock()
	defer c.sync.Unlock()

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
	c.sync.Lock()
	defer c.sync.Unlock()

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
		if _, err := c.findResource(fqdn); err == nil {
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
	// Resources that have been added to the configuration
	Added []types.Resource
	// Resources that have been updated after the parse step, typically this is
	// any change to the resource definition, but does not include changes to referenced
	// resources
	// It is possible that a resource is in both ParseUpdated and ProcessUpdated
	ParseUpdated []types.Resource
	// Resources that have been updated after the process step, typically this includes
	// any changes to referenced resources
	// It is possible that a resource is in both ParseUpdated and ProcessUpdated
	ProcessedUpdated []types.Resource
	// Resources that have been removed from the configuration
	Removed []types.Resource
	// Resources that have not changed
	Unchanged []types.Resource
}

// Diff compares the current configuration to the provided configuration and
// returns resources that have changed between the two configurations
func (c *Config) Diff(o *Config) (*ResourceDiff, error) {
	var new []types.Resource
	var parseChanged []types.Resource
	var processChanged []types.Resource
	var removed []types.Resource
	var unchanged []types.Resource

	for _, r := range o.Resources {
		// does the resource exist
		cr, err := c.findResource(r.Metadata().ID)

		// check if the resource has been found
		if err != nil {
			// resource does not exist
			new = append(new, r)
			continue
		}

		// check if the resource has changed
		if cr.Metadata().Checksum.Parsed != r.Metadata().Checksum.Parsed {
			// resource has changes rebuild
			parseChanged = append(parseChanged, r)
			continue
		}

		if cr.Metadata().Checksum.Processed != r.Metadata().Checksum.Processed {
			// resource has changes rebuild
			processChanged = append(processChanged, r)
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

		for _, r2 := range parseChanged {
			if r.Metadata().ID == r2.Metadata().ID {
				found = true
				break
			}
		}

		for _, r2 := range processChanged {
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
		Added:            new,
		Removed:          removed,
		ParseUpdated:     parseChanged,
		ProcessedUpdated: processChanged,
		Unchanged:        unchanged,
	}, nil

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

// WalkCallback is called with the resource when the graph processes that particular node
type WalkCallback func(r types.Resource) error

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

			r, ok := v.(types.Resource)
			// not a resource skip, this should never happen
			if !ok {
				panic("an item has been added to the graph that is not a resource")
			}

			// if this is the root module or is disabled skip
			if (r.Metadata().Type == types.TypeRoot || r.Metadata().Type == types.TypeModule) || r.GetDisabled() {
				return nil
			}

			// call the callback only if a previous error has not occurred
			if hasError.Load() {
				return nil
			}

			err := wf(r)
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

func (c *Config) addResource(r types.Resource, ctx *hcl.EvalContext, b *hclsyntax.Body) error {
	fqdn := types.FQDNFromResource(r)

	// set the ID
	r.Metadata().ID = fqdn.String()

	rf, err := c.findResource(fqdn.String())
	if err == nil && rf != nil {
		return ResourceExistsError{r.Metadata().Name}
	}

	c.Resources = append(c.Resources, r)
	c.contexts[r] = ctx
	c.bodies[r] = b

	return nil
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
