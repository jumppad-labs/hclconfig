package xcl

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// NewQuerier creates a new Querier for the given resource type
// This will allow you to find resources by their FQRN or type
// Querier always returns strongly typed resources
func NewQuerier[T any](c *Config) *Querier[T] {
	return &Querier[T]{config: c}
}

type Querier[T any] struct {
	config *Config
}

// FindResource finds a resource by its FQRN path
// If the resource is not found, it returns a ResourceNotFoundError
func (q *Querier[T]) FindResource(path string) (*T, error) {
	returnResource := new(T)

	// Use Config's GetResources() method
	resources := q.config.GetResources()
	for _, r := range resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			panic(err) // should never happen, all resources should have metadata
		}

		if meta.ID == path {
			err := schema.UnmarshalUntyped(r, returnResource)
			return returnResource, err
		}
	}

	// return a zero value of T and an error
	return returnResource, state.ResourceNotFoundError{Resource: path}
}

// FindResourcesByType finds all resources of the given type
// If no resources are found, it returns a ResourceNotFoundError
func (q *Querier[T]) FindResourcesByType() ([]*T, error) {
	var results []*T

	t := new(T)
	metaT, err := types.GetMeta(t)
	if err != nil {
		return nil, fmt.Errorf("unable to get metadata for type %T: %w", t, err)
	}

	// Use Config's GetResources() method
	resources := q.config.GetResources()
	for _, r := range resources {
		metaR, err := types.GetMeta(r)
		if err != nil {
			panic(err) // should never happen, all resources should have metadata
		}

		if metaR.Type == metaT.Type {
			var nr *T
			err := schema.UnmarshalUntyped(r, &nr)
			if err != nil {
				return nil, err
			}
			results = append(results, nr)
		}
	}

	if len(results) == 0 {
		return nil, state.ResourceNotFoundError{Resource: fmt.Sprintf("no resources of type %T found", new(T))}
	}

	return results, nil
}
