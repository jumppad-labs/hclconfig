package hclconfig

import (
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"strings"

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
	root := (&types.Module{}).New("root")

	graph := &dag.AcyclicGraph{}
	graph.Add(root)

	// Loop over all resources and add to dag
	for _, resource := range c.Resources {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		// add links to dependencies
		// this is here for now as we might need to process these two
		// lists separately
		for _, v := range resource.Info().ResouceLinks {
			resource.Info().DependsOn = append(resource.Info().DependsOn, v)
		}

		for _, d := range resource.Info().DependsOn {

			var err error
			dependencies := []types.Resource{}
			fqdn, err := ParseFQDN(d)
			if err != nil {
				return nil, fmt.Errorf("invalid dependency: %s, error: %s", d, err)
			}

			// only search for module dependencies when has a module path and
			// is not a resource or output
			if fqdn.Module != "" && fqdn.Resource == "" {
				deps, err := c.FindRelativeModuleResources(fqdn.Module, resource.Info().Module, true)
				if err != nil {
					return nil, fmt.Errorf("unable to find module resource, module: %s, error: %s", fqdn.Module, err)
				}

				dependencies = append(dependencies, deps...)
			}

			if fqdn.Resource != "" {
				dep, err := c.FindRelativeResource(d, resource.Info().Module)
				if err != nil {
					return nil, fmt.Errorf("unable to find resource from parent module: '%s, error: %s", resource.Info().Module, err)
				}

				dependencies = append(dependencies, dep)
			}

			for _, d := range dependencies {
				hasDeps = true
				graph.Connect(dag.BasicEdge(d, resource))
			}
		}

		// if this resource is part of a module make it depend on that module
		if resource.Info().Module != "" {
			parts := strings.Split(resource.Info().Module, ".")
			myModule := parts[(len(parts) - 1)]
			parentModule := parts[:len(parts)-1]

			fqdn := &ResourceFQDN{
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

// ParseCallback is called with the resource when the graph processes that particular node
type ParseCallback func(r types.Resource) error

// Until parse is called the HCL configuration is not deserialized into
// the structs. We have to do this using a graph as some inputs depend on
// outputs from other resrouces, therefore we need to process this is strict order
func (c *Config) process(wf ParseCallback) error {
	d, err := doYaLikeDAGs(c)
	if err != nil {
		return fmt.Errorf("unable to create graph: %s", err)
	}

	// reduce the graph nodes to unique instances
	d.TransitiveReduction()

	err = d.Validate()
	if err != nil {
		return fmt.Errorf("Unable to validate dependency graph: %w", err)
	}

	// define the walker callback that will be called for every node in the graph
	w := dag.Walker{}
	w.Callback = func(v dag.Vertex) (diags tfdiags.Diagnostics) {

		r, ok := v.(types.Resource)

		// not a resource skip, this should never happen
		if !ok {
			panic("an item has been added to the graph that is not a resource")
		}

		// attempt to set the values in the resource links to the resource attribute
		// all linked values should now have been processed as the graph
		// will have handled them first
		for _, v := range r.Info().ResouceLinks {
			fqdn, err := ParseFQDN(v)
			if err != nil {
				return diags.Append(fmt.Errorf("error parsing resource link error:%s", err))
			}

			// get the value from the linked resource
			l, err := c.FindRelativeResource(v, r.Info().Module)
			if err != nil {
				return diags.Append(fmt.Errorf("unable to find dependent resource %s, %s\n", v, err))
			}

			var src reflect.Value

			// get the type of the linked resource
			paramType := findTypeFromInterface(fqdn.Attribute, l)

			// did we find the type if not check the meta properties
			if paramType == "" {
				paramType = findTypeFromInterface(fqdn.Attribute, l.Info())
				if paramType == "" {
					return diags.Append(fmt.Errorf("type not found %v\n", fqdn.Attribute))
				}
			}

			// find the value
			path := strings.Split(fqdn.Attribute, ".")
			src, err = lookup.LookupI(l, path, []string{"hcl", "json"})

			// the property might be one of the meta properties check the resource info
			if err != nil {
				src, err = lookup.LookupI(l.Info(), path, []string{"hcl", "json"})

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

			setContextVariableFromPath(r.Info().Context, v, val)
		}

		// Process the raw resouce now we have the context from the linked
		// resources
		if r.Info().Body != nil {
			diag := gohcl.DecodeBody(r.Info().Body, r.Info().Context, r)
			if diag.HasErrors() {
				return diags.Append(fmt.Errorf(diag.Error()))
			}
		}

		// call the resource process method
		err = r.Process()
		if err != nil {
			return diags.Append(fmt.Errorf("error calling process for resource: %s", err))
		}

		// call the callbacks
		if wf != nil {
			err = wf(r)
			if err != nil {
				return diags.Append(fmt.Errorf("error processing graph node: %s", err))
			}
		}

		// if the type is a module we need to add the variables to the
		// context
		if r.Info().Type == types.TypeModule {
			mod := r.(*types.Module)

			var mapVars map[string]cty.Value
			if att, ok := mod.Variables.(*hcl.Attribute); ok {
				val, _ := att.Expr.Value(mod.Context)
				mapVars = val.AsValueMap()

				for k, v := range mapVars {
					fmt.Println(k, v)
					setContextVariable(mod.SubContext, k, v)
				}
			}
		}

		return nil
	}

	// update the dag and process the nodes
	log.SetOutput(ioutil.Discard)

	w.Update(d)
	tf := w.Wait()
	if tf.Err() != nil {
		return tf.Err()
	}

	return nil
}
