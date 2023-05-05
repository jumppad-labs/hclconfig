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

	graph := &dag.AcyclicGraph{}

	// add a root node for the graph
	root, _ := types.DefaultTypes().CreateResource(types.TypeRoot, "root")
	graph.Add(root)

	// Loop over all resources and add to graph
	for _, resource := range c.Resources {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		// if disabled ignore any dependencies
		if resource.Metadata().Disabled {
			// add all disabled resources to the root
			//fmt.Println("connect", "root", "to", resource.Metadata().ID)

			graph.Connect(dag.BasicEdge(root, resource))
			continue
		}

		// use a map to keep a unique list
		dependencies := map[types.Resource]bool{}

		// add links to dependencies
		// this is here for now as we might need to process these two lists separately
		resource.Metadata().DependsOn = append(resource.Metadata().DependsOn, resource.Metadata().ResourceLinks...)

		for _, d := range resource.Metadata().DependsOn {
			var err error
			fqdn, err := types.ParseFQRN(d)
			if err != nil {
				pe := ParserError{}
				pe.Line = resource.Metadata().Line
				pe.Column = resource.Metadata().Column
				pe.Filename = resource.Metadata().File
				pe.Message = fmt.Sprintf("invalid dependency: %s, error: %s", d, err)

				return nil, pe
			}

			// when the dependency is a module, depend on all resources in the module
			if fqdn.Type == types.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resource.Metadata().Module)
				deps, err := c.FindModuleResources(relFQDN.String(), true)
				if err != nil {
					pe := ParserError{}
					pe.Line = resource.Metadata().Line
					pe.Column = resource.Metadata().Column
					pe.Filename = resource.Metadata().File
					pe.Message = fmt.Sprintf("unable to find resources for module: %s, error: %s", fqdn.Module, err)

					return nil, pe
				}

				for _, dep := range deps {
					dependencies[dep] = true
				}
			}

			// when the dependency is a resource, depend on the resource
			if fqdn.Type != types.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resource.Metadata().Module)
				dep, err := c.FindResource(relFQDN.String())
				if err != nil {
					pe := ParserError{}
					pe.Line = resource.Metadata().Line
					pe.Column = resource.Metadata().Column
					pe.Filename = resource.Metadata().File
					pe.Message = fmt.Sprintf("unable to find dependent resource in module: '%s', error: '%s'", resource.Metadata().Module, err)

					return nil, pe
				}

				dependencies[dep] = true
			}
		}

		// if this resource is part of a module make it depend on that module
		if resource.Metadata().Module != "" {
			fqdnString := fmt.Sprintf("module.%s", resource.Metadata().Module)

			d, err := c.FindResource(fqdnString)
			if err != nil {
				pe := ParserError{}
				pe.Line = resource.Metadata().Line
				pe.Column = resource.Metadata().Column
				pe.Filename = resource.Metadata().File
				pe.Message = fmt.Sprintf("unable to find resources parent module: '%s, error: %s", fqdnString, err)

				return nil, pe
			}

			hasDeps = true
			dependencies[d] = true
		}

		for d := range dependencies {
			hasDeps = true
			//fmt.Println("connect", resource.Metadata().ID, "to", d.Metadata().ID)
			graph.Connect(dag.BasicEdge(d, resource))
		}

		// if no deps add to root node
		if !hasDeps {
			//fmt.Println("connect", resource.Metadata().ID, "to root")
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
			if (r.Metadata().Type == types.TypeRoot || r.Metadata().Type == types.TypeModule) || r.Metadata().Disabled {
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
		return err
	}

	// reduce the graph nodes to unique instances
	d.TransitiveReduction()

	// validate the dependency graph is ok
	err = d.Validate()
	if err != nil {
		return fmt.Errorf("unable to validate dependency graph: %w", err)
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
		if (r.Metadata().Type == types.TypeRoot) || r.Metadata().Disabled {
			return nil
		}

		bdy, err := c.getBody(r)
		if err != nil {
			panic(fmt.Sprintf(`no body found for resource "%s"`, r.Metadata().ID))
		}

		ctx, err := c.getContext(r)
		if err != nil {
			panic("no context found for resource")
		}

		// attempt to set the values in the resource links to the resource attribute
		// all linked values should now have been processed as the graph
		// will have handled them first
		for _, v := range r.Metadata().ResourceLinks {
			fqdn, err := types.ParseFQRN(v)
			if err != nil {
				pe := ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf("error parsing resource link %s", err)

				return diags.Append(pe)
			}

			// get the value from the linked resource
			l, err := c.FindRelativeResource(v, r.Metadata().Module)
			if err != nil {
				pe := ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`unable to find dependent resource "%s" %s`, v, err)

				return diags.Append(pe)
			}

			// if the type is output always look for value
			if fqdn.Type == types.TypeOutput {
				fqdn.Attribute = "value"
			}

			var src reflect.Value

			// get the type of the linked resource
			paramType := findTypeFromInterface(fqdn.Attribute, l)

			// did we find the type if not check the meta properties
			if paramType == "" {
				paramType = findTypeFromInterface(fqdn.Attribute, l.Metadata())
				if paramType == "" {
					pe := ParserError{}
					pe.Filename = r.Metadata().File
					pe.Line = r.Metadata().Line
					pe.Column = r.Metadata().Column
					pe.Message = fmt.Sprintf(`type not found "%v"`, fqdn.Attribute)

					return diags.Append(pe)
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
					pe := ParserError{}
					pe.Filename = r.Metadata().File
					pe.Line = r.Metadata().Line
					pe.Column = r.Metadata().Column
					pe.Message = fmt.Sprintf(`value not found "%s" %s`, fqdn.Attribute, err)

					return diags.Append(pe)
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
				pe := ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`pointer values are not implemented "%v"`, src)

				return diags.Append(pe)
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
				pe := ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`unable to link resource "%s" as it references an unsupported type "%s"`, v, paramType)

				return diags.Append(pe)
			}

			setContextVariableFromPath(ctx, v, val)
		}

		// Process the raw resource now we have the context from the linked
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
				pe := ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`unable to create resource "%s" %s`, r.Metadata().ID, err)

				return diags.Append(pe)
			}
		}

		// call the callbacks
		if wf != nil {
			err := wf(r)
			if err != nil {
				pe := ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`unable to create resource "%s" %s`, r.Metadata().ID, err)

				return diags.Append(pe)
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
