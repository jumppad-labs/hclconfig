// Package plugins provides interfaces and types for implementing HCL resource providers.
package plugins

import (
	"context"

	"github.com/jumppad-labs/hclconfig/types"
)

// Jumppad uses a plugin model that allows you to register custom providers
// each plugin can register can register multiple providers where
// each provider is responsible for the lifecycle of a single type of resource.

// ResourceProvider defines the generic interface that all resource providers must implement.
// It provides lifecycle management for resources including creation, destruction,
// refresh, and state checking operations.
// T must be a type that implements types.Resource.
type ResourceProvider[T types.Resource] interface {
	// Init initializes the provider with state access, provider functions, and a logger.
	// This method is called once when the provider is created and should be used
	// to set up any required clients or dependencies.
	//
	// The state parameter provides access to the current state of resources.
	// The functions parameter provides access to provider-defined functions.
	// The logger parameter is the logger instance for all logging operations.
	Init(state State, functions ProviderFunctions, logger Logger) error

	// Create creates a new resource or recreates a failed resource.
	// This method is called when a resource does not exist or when creation
	// has previously failed and 'up' is executed.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The resource parameter contains the resource configuration to create.
	// Returns the created resource with updated state and any creation error.
	//
	// The implementation should periodically check the context for cancellation
	// and return promptly if the context is cancelled.
	Create(ctx context.Context, resource T) (T, error)

	// Destroy removes an existing resource.
	// This method is called when a resource exists and 'down' is executed,
	// or when cleanup is required after a failure.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The resource parameter contains the resource configuration to destroy.
	// The force parameter, when true, indicates resources should be destroyed quickly
	// without waiting for graceful shutdown of long-running operations.
	//
	// The implementation should periodically check the context for cancellation.
	Destroy(ctx context.Context, resource T, force bool) error

	// Refresh updates the state of an existing resource.
	// This method is called when a resource exists and 'up' is executed
	// to ensure the resource is in the desired state.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The resource parameter contains the resource configuration to refresh.
	//
	// The implementation should periodically check the context for cancellation.
	Refresh(ctx context.Context, resource T) error

	// Changed determines if a resource has changed since the last operation.
	// This method is used to detect drift and determine if a resource
	// needs to be updated or recreated.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The resource parameter contains the resource configuration to check.
	// Returns true if the resource has changed, false otherwise, and any error
	// encountered while checking for changes.
	Changed(ctx context.Context, resource T) (bool, error)

	// Functions returns the functions exposed by the provider that can be called
	// by other providers.
	Functions() ProviderFunctions
}

