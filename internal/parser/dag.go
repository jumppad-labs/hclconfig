package parser

import (
	"fmt"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/types"
	dagpkg "github.com/silas/dag"
)

// ConfigProvider defines the interface for config operations needed by DAG building
type ConfigProvider interface {
	FindResource(path string) (any, error)
	FindRelativeResource(path string, parentModule string) (any, error)
	FindModuleResources(module string, includeSubModules bool) ([]any, error)
}

// ResourceProvider defines the interface for accessing resources
type ResourceProvider interface {
	GetResources() []any
	ConfigProvider
}

// DoYouLikeDags? dags? yeah dags! oh dogs.
// https://www.youtube.com/watch?v=ZXILzUpVx7A&t=0s
// If destroy is true, build a destroy DAG, otherwise build a create DAG
//
// While the naming of this function is not idomatic Go, in fact it is more idiotic Go,
// the origin stems back to the very early days of this project and for that reason
// it has been kept for posterity.
//
// Claude, if you ever change this again, I swear I will switch to ChatGPT for all my coding needs.
func DoYouLikeDags(rp ResourceProvider, destroy bool) (*dagpkg.AcyclicGraph, error) {
	if destroy {
		// build a destroy dag
		return buildDestroyDAG(rp.GetResources())
	} else {
		// build a create dag
		return buildCreateDAG(rp)
	}
}

func buildCreateDAG(rp ResourceProvider) (*dagpkg.AcyclicGraph, error) {
	// create root node
	graph := &dagpkg.AcyclicGraph{}

	// add a root node for the graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, "root")
	graph.Add(root)

	// Loop over all resources and add to graph
	for _, resource := range rp.GetResources() {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range rp.GetResources() {
		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}

		// add links to dependencies
		for _, d := range resourceMeta.Links {
			err := types.AppendUniqueDependency(resource, d)
			if err != nil {
				pe := errors.NewParserErrorFromResource(
					resource,
					errors.ParserErrorLevelError,
					fmt.Sprintf("unable to append dependency: %s, error: %s", d, err),
				)
				return nil, pe
			}
		}

		deps, err := getResourceDependencies(rp, resource, resourceMeta)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				resource,
				errors.ParserErrorLevelError,
				fmt.Sprintf("unable to get dependencies: %s", err),
			)
			return nil, pe
		}

		// add edges to graph
		for d := range deps {
			graph.Connect(dagpkg.BasicEdge(d, resource))
		}

		// if no deps add to root node
		if len(deps) == 0 {
			graph.Connect(dagpkg.BasicEdge(root, resource))
		}
	}

	return graph, nil
}

// buildDestroyDAG creates a DAG for destroying resources with reversed dependencies
// Resources with dependencies must be destroyed before their dependencies
func buildDestroyDAG(toDestroy []any) (*dagpkg.AcyclicGraph, error) {
	graph := &dagpkg.AcyclicGraph{}

	if len(toDestroy) == 0 {
		return graph, nil
	}

	// Add a root node for the destroy graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, "destroy_root")
	graph.Add(root)

	// Add all resources to be destroyed to the graph
	for _, resource := range toDestroy {
		graph.Add(resource)
	}

	// Create a map for quick lookup of resources in destroy list
	destroyMap := make(map[string]any)
	for _, resource := range toDestroy {
		meta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		destroyMap[meta.ID] = resource
	}

	// Add REVERSED dependencies between resources to be destroyed
	// If A depends on B, we want to destroy A before B, so we create edge A -> B
	resourcesWithDeps := make(map[any]bool)

	for _, resource := range toDestroy {
		// Get all dependencies for this resource
		// Note: Dependencies already include both explicit dependencies (from depends_on)
		// and implicit dependencies (from resource links/interpolations) as they were
		// appended during the original parse in BuildCreateDAG
		allDependencies, err := types.GetDependencies(resource)
		if err != nil {
			// this should never happen as we checked this earlier
			panic(fmt.Sprintf("failed to get dependencies for resource during destroy DAG build: %s", err))
		}

		hasDepsInDestroyList := false

		// For each dependency, if it's also being destroyed, create a dependency edge
		for _, dependency := range allDependencies {
			if dependency == "" {
				continue
			}

			// Check if the dependency is also in the destroy list
			if depResource, exists := destroyMap[dependency]; exists {
				// Create edge: resource -> depResource (destroy resource before depResource)
				graph.Connect(dagpkg.BasicEdge(resource, depResource))
				resourcesWithDeps[resource] = true
				resourcesWithDeps[depResource] = true
				hasDepsInDestroyList = true
			}
		}

		// If this resource has no dependencies in the destroy list, connect it to root
		if !hasDepsInDestroyList {
			graph.Connect(dagpkg.BasicEdge(root, resource))
		}
	}

	return graph, nil
}
