package hclconfig

import (
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform/dag"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/kr/pretty"
	"github.com/shipyard-run/hclconfig/lookup"
	"github.com/shipyard-run/hclconfig/types"
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
		//fmt.Printf("Resource: %s, Type: %s\n", resource.Info().Name, resource.Info().Type)
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

// WalkFunc is called with the resource when the graph processes that particular node
type WalkFunc func(r types.Resource) error

func (c *Config) Walk(wf WalkFunc) error {
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

		// attempt to set the values in the resource links to the resource
		// all linked values should now have been processed as the graph
		// will have handled them first
		for k, v := range r.Info().ResouceLinks {
			// get the resource where the value exists
			parts := strings.Split(v, ".")
			n := parts[1:3]
			sn := strings.Join(n, ".")

			// get the value path
			valPath := parts[3:]

			// get the value from the linked resource
			l, _ := c.FindResource(sn)

			var src reflect.Value
			var err error

			src, err = lookup.LookupI(l, valPath, []string{"hcl", "json"})

			if err != nil {
				// the property might be one of the meta properties check the resource info
				src, err = lookup.LookupI(l.Info(), valPath, []string{"hcl", "json"})
				if err != nil {
					panic(fmt.Sprintf("value not found %s, %s, %s\n", valPath, pretty.Sprint(l), err))
				}
			}

			err = lookup.SetValueStringI(r, src, k, []string{"hcl", "json"})
			if err != nil {
				return diags.Append(err)
			}
		}

		// call the callback
		err := wf(r)
		if err != nil {
			return diags.Append(fmt.Errorf("error processing graph node: %s", err))
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
