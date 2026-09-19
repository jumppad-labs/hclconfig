package xcl

import (
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
//
// Resources of a type registered with PluginRegistry.RegisterType are held in
// state as T, they are returned by reference, changing the returned value
// changes the value held in state. Plugin resources are returned as a copy.
func (q *Querier[T]) FindResource(path string) (*T, error) {
	// Use Config's GetResources() method
	resources := q.config.GetResources()
	for _, r := range resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			panic(err) // should never happen, all resources should have metadata
		}

		if meta.ID == path {
			return asType[T](r)
		}
	}

	// return a zero value of T and an error
	return new(T), state.ResourceNotFoundError{Resource: path}
}

// FindResourcesByType finds all resources whose type is typeName, i.e. the
// "postgres" in resource "postgres" "main"
// If no resources are found, it returns a ResourceNotFoundError
//
// Resources of a type registered with PluginRegistry.RegisterType are held in
// state as T, they are returned by reference, changing a returned value
// changes the value held in state. Plugin resources are returned as copies.
func (q *Querier[T]) FindResourcesByType(typeName string) ([]*T, error) {
	var results []*T

	// Use Config's GetResources() method
	resources := q.config.GetResources()
	for _, r := range resources {
		meta, err := types.GetMeta(r)
		if err != nil {
			panic(err) // should never happen, all resources should have metadata
		}

		if meta.Type == typeName {
			typed, err := asType[T](r)
			if err != nil {
				return nil, err
			}

			results = append(results, typed)
		}
	}

	if len(results) == 0 {
		return nil, state.ResourceNotFoundError{Resource: typeName}
	}

	return results, nil
}

// asType returns r as a *T. A resource that already is a *T, a registered
// type, is returned as is. Any other resource, such as the schema-generated
// instance of a plugin type, is copied into a new T.
func asType[T any](r any) (*T, error) {
	if typed, ok := r.(*T); ok {
		return typed, nil
	}

	typed := new(T)
	err := schema.UnmarshalUntyped(r, typed)
	return typed, err
}
