package hclconfig

import (
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl2/gohcl"
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
			parts := strings.Split(v, ".")
			n := parts[1:3]
			sn := strings.Join(n, ".")

			resource.Info().DependsOn = append(resource.Info().DependsOn, sn)
		}

		for _, d := range resource.Info().DependsOn {
			var err error
			dependencies := []types.Resource{}

			if strings.HasPrefix(d, "module.") {
				// find dependencies from modules
				dependencies, err = c.FindModuleResources(d)
				if err != nil {
					return nil, err
				}
			} else {
				// find dependencies for direct resources
				r, err := c.FindResource(d)
				if err != nil {
					return nil, err
				}
				dependencies = append(dependencies, r)
			}

			for _, d := range dependencies {
				hasDeps = true
				graph.Connect(dag.BasicEdge(d, resource))
			}
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
			// get the resource where the value exists
			parts := strings.Split(v, ".")
			n := parts[1:3]
			sn := strings.Join(n, ".")

			// get the value path
			valPath := parts[3:]

			// get the value from the linked resource
			l, err := c.FindResource(sn)
			if err != nil {
				return diags.Append(fmt.Errorf("unable to find dependent resource %s, %s\n", sn, err))
			}

			var src reflect.Value

			// get the type of the linked resource
			paramType := findTypeFromInterface(strings.Join(valPath, "."), l)

			// did we find the type if not check the meta properties
			if paramType == "" {
				paramType = findTypeFromInterface(strings.Join(valPath, "."), l.Info())
				if paramType == "" {
					return diags.Append(fmt.Errorf("type not found %v\n", valPath))
				}
			}

			// find the value
			src, err = lookup.LookupI(l, valPath, []string{"hcl", "json"})

			// the property might be one of the meta properties check the resource info
			if err != nil {
				src, err = lookup.LookupI(l.Info(), valPath, []string{"hcl", "json"})

				// still not found return an error
				if err != nil {
					return diags.Append(fmt.Errorf("value not found %s, %s\n", valPath, err))
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
