package plugins

import (
	"context"
	"encoding/json"

	"github.com/jumppad-labs/hclconfig/types"
)

// ProviderAdapter defines a common interface for all providers regardless of their concrete type.
//
// This adapter pattern exists to bridge the gap between the plugin system's dynamic nature
// and Go's static type system. The plugin system must work with:
//   - String-based type identification ("resource.container", "resource.network", etc.)
//   - Raw []byte data from HCL configuration files
//   - Runtime provider lookup and invocation
//
// However, ResourceProvider[T] implementations work with strongly-typed resources and
// provide compile-time type safety. The ProviderAdapter interface allows the plugin
// system to work uniformly with all provider types without knowing their concrete types.
//
// Without this adapter, the plugin system would need to use reflection or type assertions
// to convert between []byte and concrete types, making the code complex and error-prone.
type ProviderAdapter interface {
	Validate(ctx context.Context, entityData []byte) error
	Create(ctx context.Context, entityData []byte) error
	Destroy(ctx context.Context, entityData []byte, force bool) error
	Refresh(ctx context.Context, entityData []byte) error
	Update(ctx context.Context, entityData []byte) error
	Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error)
}

// TypedProviderAdapter wraps a ResourceProvider[T] to implement ProviderAdapter.
//
// This adapter serves as a type-safe bridge between the generic plugin system and
// strongly-typed resource providers. It performs the following key functions:
//
//  1. Type Conversion: Converts []byte configuration data to concrete Go types (T)
//     using the stored schema and unmarshaling logic.
//
//  2. Method Translation: Translates ProviderAdapter method calls to ResourceProvider[T]
//     method calls, handling the type conversion seamlessly.
//
//  3. Error Handling: Provides consistent error handling and validation across all
//     provider types without requiring each provider to handle []byte parsing.
//
//  4. State Management: Manages provider initialization and state consistently across
//     all provider implementations.
//
// Why not use ResourceProvider[T] directly in the plugin system?
//   - The plugin system works with runtime type resolution (string -> provider lookup)
//   - Configuration data comes as []byte from HCL files, not as typed structs
//   - Each ResourceProvider[T] has a different concrete type T, making uniform handling impossible
//   - This adapter provides a uniform interface while preserving type safety within providers
//
// Example flow:
//
//	Plugin receives: ("resource", "container", []byte{...})
//	Adapter unmarshals: []byte -> *ContainerResource
//	Provider receives: ResourceProvider[*ContainerResource].Create(ctx, *ContainerResource)
type TypedProviderAdapter[T types.Resource] struct {
	provider     ResourceProvider[T]
	concreteType T
	state        State
	functions    ProviderFunctions
	logger       Logger
}

// NewTypedProviderAdapter creates a new adapter for a typed provider
func NewTypedProviderAdapter[T types.Resource](provider ResourceProvider[T], concreteType T) *TypedProviderAdapter[T] {
	return &TypedProviderAdapter[T]{
		provider:     provider,
		concreteType: concreteType,
	}
}

func (a *TypedProviderAdapter[T]) Init(state State, functions ProviderFunctions, logger Logger) error {
	a.state = state
	a.functions = functions
	a.logger = logger
	return a.provider.Init(state, functions, logger)
}

func (a *TypedProviderAdapter[T]) Validate(ctx context.Context, entityData []byte) error {
	// Create a new instance of type T to unmarshal into
	var resource T

	// Try to unmarshal JSON bytes into the concrete type
	// This serves as basic validation - if it can't unmarshal, it's invalid
	if err := json.Unmarshal(entityData, &resource); err != nil {
		return err
	}

	// Additional validation could be added here by calling a Validate method
	// on the resource if it implements a Validator interface
	
	return nil
}

func (a *TypedProviderAdapter[T]) Create(ctx context.Context, entityData []byte) error {
	// Create a new instance of type T to unmarshal into
	var resource T

	// Unmarshal JSON bytes into the concrete type
	if err := json.Unmarshal(entityData, &resource); err != nil {
		return err
	}

	// Call the provider's Create method with the concrete type
	// (provider was already initialized during registration)
	_, err := a.provider.Create(ctx, resource)
	return err
}

func (a *TypedProviderAdapter[T]) Destroy(ctx context.Context, entityData []byte, force bool) error {
	// Create a new instance of type T to unmarshal into
	var resource T

	// Unmarshal JSON bytes into the concrete type
	if err := json.Unmarshal(entityData, &resource); err != nil {
		return err
	}

	// Call the provider's Destroy method with the concrete type
	// (provider was already initialized during registration)
	return a.provider.Destroy(ctx, resource, force)
}

func (a *TypedProviderAdapter[T]) Refresh(ctx context.Context, entityData []byte) error {
	// Create a new instance of type T to unmarshal into
	var resource T

	// If entityData is provided, unmarshal it
	if entityData != nil {
		// Unmarshal JSON bytes into the concrete type
		if err := json.Unmarshal(entityData, &resource); err != nil {
			return err
		}
	}

	// Call the provider's Refresh method with the concrete type
	// Note: if entityData was nil, resource will be the zero value of T
	return a.provider.Refresh(ctx, resource)
}

func (a *TypedProviderAdapter[T]) Update(ctx context.Context, entityData []byte) error {
	// Create a new instance of type T to unmarshal into
	var resource T

	// Unmarshal JSON bytes into the concrete type
	if err := json.Unmarshal(entityData, &resource); err != nil {
		return err
	}

	// Call the provider's Update method with the concrete type
	return a.provider.Update(ctx, resource)
}

func (a *TypedProviderAdapter[T]) Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error) {
	// Create instances for old and new resources
	var oldResource, newResource T

	// Unmarshal old resource data
	if err := json.Unmarshal(oldEntityData, &oldResource); err != nil {
		return false, err
	}

	// Unmarshal new resource data
	if err := json.Unmarshal(newEntityData, &newResource); err != nil {
		return false, err
	}

	// Call the provider's Changed method with both resources
	return a.provider.Changed(ctx, oldResource, newResource)
}
