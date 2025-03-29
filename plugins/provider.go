package plugins

import (
	"context"

	"github.com/jumppad-labs/hclconfig/types"
)

type Provider interface {
	// Init is called when the provider is created, it is passed a logger that
	// can be used for any logging purposes. Any other clients must be created
	// by the provider
	//
	// cfg is the configuration for the provider that has been deserialized
	// from the configuration file
	// log is the logger that the provider should use for any logging purposes
	Init(cfg types.Resource, log Logger) error

	// Create is called when a resource does not exist or creation has previously
	// failed and 'up' is run
	//
	// The context is used to cancel the operation if it takes too long
	// or the user cancels the operation. The plugin should check the context
	// periodically to see if it has been cancelled
	Create(ctx context.Context) error

	// Destroy is called when a resource is failed or created and 'down' is run
	//
	// The context is used to cancel the operation if it takes too long
	// or the user cancels the operation. The plugin should check the context
	// periodically to see if it has been cancelled
	// force true indicates that the resource should be destroyed quickly and
	// without waiting for any long running operations to complete
	Destroy(ctx context.Context, force bool) error

	// Refresh is called when a resource is created and 'up' is run
	//
	// The context is used to cancel the operation if it takes too long
	// or the user cancels the operation. The plugin should check the context
	// periodically to see if it has been cancelled
	// force true indicates that the resource should be destroyed quickly and
	// without waiting for any long running operations to complete
	Refresh(ctx context.Context) error

	// Changed returns if a resource has changed since the last run
	Changed() (bool, error)

	// Lookup is a utility to determine the existence of a resource
	Lookup() ([]string, error)
}
