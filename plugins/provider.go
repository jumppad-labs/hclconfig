// Package plugins provides interfaces and types for implementing HCL resource providers.
package plugins

import (
	"context"
)

// Jumppad uses a plugin model that allows you to register custom providers
// each plugin can register can register multiple providers where
// each provider is responsible for the lifecycle of a single type of resource.

// ResourceProvider defines the generic interface that all resource providers must implement.
// It provides lifecycle management for resources including creation, destruction,
// reading, and change detection operations.
// T must be a type that has embedded types.ResourceBase.
type ResourceProvider[T any] interface {
	// Init initializes the provider with state access, provider functions, and a logger.
	// This method is called once when the provider is created and should be used
	// to set up any required clients or dependencies.
	//
	// The state parameter provides access to the current state of resources.
	// The functions parameter provides access to provider-defined functions.
	// The logger parameter is the logger instance for all logging operations.
	Init(state State, functions ProviderFunctions, logger Logger) error

	// Create creates a new resource.
	// This method is called when a resource is not in the previous state, when
	// Read reports the resource as not found, and after a resource saved as failed
	// has been destroyed (xcl destroys a failed resource, then creates it again).
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
	// or before a resource saved as failed is created again.
	//
	// Whether destroying a resource that no longer exists is an error is the
	// provider's decision. Any error returned fails the destroy.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The resource parameter contains the resource configuration to destroy.
	// The force parameter, when true, indicates resources should be destroyed quickly
	// without waiting for graceful shutdown of long-running operations.
	//
	// The implementation should periodically check the context for cancellation.
	Destroy(ctx context.Context, resource T, force bool) error

	// Read reports the real resource.
	// This method is only called for resources that are in the previous state,
	// so old is never nil. Read is called before Changed.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The old parameter is the resource as saved by the last apply. Use it to
	// locate the real resource, for example by an ID set at create.
	// The new parameter is the resource as described by the current configuration.
	// Returns new with its identity, observed and derived fields filled in from
	// the real resource.
	//
	// Read must never change configured fields, must never record values that
	// change on their own (such as uptime or timestamps), and must never create,
	// change or remove anything in the real world.
	//
	// When the real resource no longer exists, Read returns ErrNotFound and xcl
	// creates the resource again. Any other error fails the apply.
	//
	// The implementation should periodically check the context for cancellation.
	Read(ctx context.Context, old T, new T) (T, error)

	// Update updates an existing resource to match the desired configuration.
	// This method is called after Changed() returns true, indicating the resource
	// needs to be updated.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The resource parameter contains the desired resource configuration.
	// Returns the updated resource with new state and any update error.
	//
	// The implementation should periodically check the context for cancellation.
	Update(ctx context.Context, resource T) (T, error)

	// Changed determines if a resource has changed by comparing the current state
	// with the desired configuration.
	//
	// The ctx parameter provides cancellation and timeout control.
	// The old parameter contains the resource as saved by the last apply.
	// The new parameter contains the current configuration after it has been
	// through Read, so it holds both configuration edits and observed drift.
	//
	// Embed DefaultChanged in the provider to get a default comparison;
	// defining Changed on the provider overrides it.
	// Returns true if the resource has changed and needs updating, false otherwise,
	// and any error encountered while checking for changes.
	Changed(ctx context.Context, old T, new T) (bool, error)

	// Functions returns the functions exposed by the provider that can be called
	// by other providers.
	Functions() ProviderFunctions
}
