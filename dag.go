package hclconfig

import (
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/terraform/dag"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/shipyard-run/hclconfig/lookup"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)

// doYaLikeDAGs? dags? yeah dags! oh dogs.
// https://www.youtube.com/watch?v=ZXILzUpVx7A&t=0s
func doYaLikeDAGs(c *Config) (*dag.AcyclicGraph, error) {
	// create root node
	root, _ := types.DefaultTypes().CreateResource(types.TypeModule, "root")

	graph := &dag.AcyclicGraph{}
	graph.Add(root)

	// Loop over all resources and add to dag
	for _, resource := range c.Resources {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		// if disabled ignore any dependencies
		if resource.Metadata().Disabled {
			// add all disabled resources to the root
			graph.Connect(dag.BasicEdge(root, resource))
			continue
		}

		// add links to dependencies
		// this is here for now as we might need to process these two
		// lists separately
		resource.Metadata().DependsOn = append(resource.Metadata().DependsOn, resource.Metadata().ResourceLinks...)

		// use a map to keep a unique list
		dependencies := map[types.Resource]bool{}
		for _, d := range resource.Metadata().DependsOn {
			var err error
			fqdn, err := types.ParseFQDN(d)
			if err != nil {
				return nil, fmt.Errorf("invalid dependency: %s, error: %s", d, err)
			}

			// only search for module dependencies when has a module path and
			// is not a resource or output
			if fqdn.Module != "" && fqdn.Resource == "" {
				deps, err := c.FindRelativeModuleResources(d, resource.Metadata().Module, true)
				if err != nil {
					return nil, fmt.Errorf("unable to find resources for module: %s, error: %s", fqdn.Module, err)
				}

				for _, d := range deps {
					dependencies[d] = true
				}
			}

			if fqdn.Resource != "" {
				dep, err := c.FindRelativeResource(d, resource.Metadata().Module)
				if err != nil {
					return nil, fmt.Errorf("unable to find dependent resource in module: '%s', error: '%s'", resource.Metadata().Module, err)
				}

				dependencies[dep] = true
			}
		}

		for d := range dependencies {
			hasDeps = true
			graph.Connect(dag.BasicEdge(d, resource))
		}

		// if this resource is part of a module make it depend on that module
		if resource.Metadata().Module != "" {
			parts := strings.Split(resource.Metadata().Module, ".")
			myModule := parts[(len(parts) - 1)]
			parentModule := parts[:len(parts)-1]

			fqdn := &types.ResourceFQDN{
				Module:   strings.Join(parentModule, "."),
				Resource: myModule,
				Type:     types.TypeModule,
			}

			d, err := c.FindResource(fqdn.String())
			if err != nil {
				return nil, fmt.Errorf("unable to find resources parent module: '%s, error: %s", fqdn.String(), err)
			}

			hasDeps = true
			graph.Connect(dag.BasicEdge(d, resource))
		}

		// if no deps add to root node
		if !hasDeps {
			graph.Connect(dag.BasicEdge(root, resource))
		}
	}

	return graph, nil
}

// ProcessCallback is called with the resource when the graph processes that particular node
type ProcessCallback func(r types.Resource) error

// Process creates a Directed Acyclic Graph for the configuration resources depending on their
// links and references. All the resources defined in the graph are traversed and
// the provided callback is executed for every resource in the graph.
//
// Any error returned from the ProcessCallback function halts execution of any other 
// callback for resources in the graph.
//
// Specifying the reverse option to 'true' causes the graph to be traversed in reverse
// order.
func (c *Config) Process(wf ProcessCallback, reverse bool) error {
	// We need to ensure that Process does not execute the callback when
	// any other callback returns an error.
	// Unfortunately returning an error with tfdiags does not stop the walk
	hasError := atomic.Bool{}

	return c.process(
		func(v dag.Vertex) (diags tfdiags.Diagnostics) {

			r, ok := v.(types.Resource)
			// not a resource skip, this should never happen
			if !ok {
				panic("an item has been added to the graph that is not a resource")
			}

			// if this is the root module or is disabled skip
			if (r.Metadata().Name == "root" && r.Metadata().Module == "" && r.Metadata().Type == types.TypeModule) || r.Metadata().Disabled {
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
}

// Until parse is called the HCL configuration is not deserialized into
// the structs. We have to do this using a graph as some inputs depend on
// outputs from other resources, therefore we need to process this is strict order
func (c *Config) process(wf dag.WalkFunc, reverse bool) error {
	// build the graph
	d, err := doYaLikeDAGs(c)
	if err != nil {
		return fmt.Errorf("unable to create graph: %s", err)
	}

	// reduce the graph nodes to unique instances
	d.TransitiveReduction()

	// validate the dependency graph is ok
	err = d.Validate()
	if err != nil {
		return fmt.Errorf("Unable to validate dependency graph: %w", err)
	}

	// define the walker callback that will be called for every node in the graph
	w := dag.Walker{}
	w.Callback = wf
	w.Reverse = reverse

	// update the dag and process the nodes
	log.SetOutput(ioutil.Discard)

	w.Update(d)
	diags := w.Wait()
	if diags.HasErrors() {
		err := diags.Err()
		return err
	}

	return nil
}

func (c *Config) createCallback(wf ProcessCallback) func(v dag.Vertex) (diags tfdiags.Diagnostics) {
	return func(v dag.Vertex) (diags tfdiags.Diagnostics) {

		r, ok := v.(types.Resource)
		// not a resource skip, this should never happen
		if !ok {
			panic("an item has been added to the graph that is not a resource")
		}

		// if this is the root module or is disabled skip
		if (r.Metadata().Name == "root" && r.Metadata().Module == "" && r.Metadata().Type == types.TypeModule) || r.Metadata().Disabled {
			return nil
		}

		bdy, err := c.getBody(r)
		if err != nil {
			panic("no body found for resource")
		}

		ctx, err := c.getContext(r)
		if err != nil {
			panic("no context found for resource")
		}

		// attempt to set the values in the resource links to the resource attribute
		// all linked values should now have been processed as the graph
		// will have handled them first
		for _, v := range r.Metadata().ResourceLinks {
			fqdn, err := types.ParseFQDN(v)
			if err != nil {
				return diags.Append(fmt.Errorf("error parsing resource link error:%s", err))
			}

			// get the value from the linked resource
			l, err := c.FindRelativeResource(v, r.Metadata().Module)
			if err != nil {
				return diags.Append(fmt.Errorf("unable to find dependent resource %s, %s\n", v, err))
			}

			var src reflect.Value

			// get the type of the linked resource
			paramType := findTypeFromInterface(fqdn.Attribute, l)

			// did we find the type if not check the meta properties
			if paramType == "" {
				paramType = findTypeFromInterface(fqdn.Attribute, l.Metadata())
				if paramType == "" {
					return diags.Append(fmt.Errorf("type not found %v\n", fqdn.Attribute))
				}
			}

			// find the value
			path := strings.Split(fqdn.Attribute, ".")
			src, err = lookup.LookupI(l, path, []string{"hcl", "json"})

			// the property might be one of the meta properties check the resource info
			if err != nil {
				src, err = lookup.LookupI(l.Metadata(), path, []string{"hcl", "json"})

				// still not found return an error
				if err != nil {
					return diags.Append(fmt.Errorf("value not found %s, %s\n", fqdn.Attribute, err))
				}
			}

			// we need to set src in the context
			var val cty.Value
			switch paramType {
			case "string":
				val = cty.StringVal(src.String())
			case "int":
				val = cty.NumberIntVal(src.Int())
			case "bool":
				val = cty.BoolVal(src.Bool())
			case "ptr":
				return diags.Append(fmt.Errorf("pointer values are not implemented %v", src))
			case "[]string":
				vals := []cty.Value{}
				for i := 0; i < src.Len(); i++ {
					vals = append(vals, cty.StringVal(src.Index(i).String()))
				}

				val = cty.SetVal(vals)
			case "[]int":
				vals := []cty.Value{}
				for i := 0; i < src.Len(); i++ {
					vals = append(vals, cty.NumberIntVal(src.Index(i).Int()))
				}

				val = cty.SetVal(vals)
			default:
				return diags.Append(fmt.Errorf("unable to link resource %s as it references an unsupported type %s", v, paramType))
			}

			setContextVariableFromPath(ctx, v, val)
		}

		// Process the raw resouce now we have the context from the linked
		// resources
		ul := getContextLock(ctx)
		defer ul()

		diag := gohcl.DecodeBody(bdy, ctx, r)
		if diag.HasErrors() {
			return appendDiagnostic(diags, diag)
		}

		// if the config implements the processable interface call the resource process method
		if p, ok := r.(types.Processable); ok {
			err := p.Process()
			if err != nil {
				fqdn := &types.ResourceFQDN{Module: r.Metadata().Module, Type: r.Metadata().Type, Resource: r.Metadata().Name}
				return diags.Append(fmt.Errorf("error calling process for resource: %s, %s", fqdn, err))
			}
		}
		//err := r.Process()
		//if err != nil {
		//	return diags.Append(fmt.Errorf("error calling process for resource: %s", err))
		//}

		// call the callbacks
		if wf != nil {
			err := wf(r)
			if err != nil {
				return diags.Append(fmt.Errorf("error processing graph node: %s", err))
			}
		}

		// if the type is a module we need to add the variables to the
		// context
		if r.Metadata().Type == types.TypeModule {
			mod := r.(*types.Module)

			var mapVars map[string]cty.Value
			if att, ok := mod.Variables.(*hcl.Attribute); ok {
				val, _ := att.Expr.Value(ctx)
				mapVars = val.AsValueMap()

				for k, v := range mapVars {
					setContextVariable(mod.SubContext, k, v)
				}
			}
		}

		return nil
	}
}

func appendDiagnostic(tf tfdiags.Diagnostics, diags hcl.Diagnostics) tfdiags.Diagnostics {
	for _, d := range diags {
		tf = tf.Append(d)
	}

	return tf
}
