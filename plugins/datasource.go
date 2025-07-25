package plugins

import (
	"context"
)

// DataSourceProvider defines the generic interface that all data source providers must implement.
// Data sources are read-only resources that fetch external data and return it as a typed resource.
// T must be a type that has embedded types.ResourceBase.
type DataSourceProvider[T any] interface {
	// Init initializes the provider with state access, provider functions, and a logger.
	// This method is called once when the provider is created and should be used
	// to set up any required clients or dependencies.
	//
	// The state parameter provides access to the current state of resources.
	// The functions parameter provides access to provider-defined functions.
	// The logger parameter is the logger instance for all logging operations.
	Init(state State, functions ProviderFunctions, logger Logger) error

	// Refresh fetches external data and returns it as a typed resource.
	// This method is called to retrieve the latest data from the external source.
	//
	// The ctx parameter provides cancellation and timeout control.
	// Returns the fetched data as a typed resource or an error if the fetch fails.
	//
	// The implementation should periodically check the context for cancellation
	// and return promptly if the context is cancelled.
	Refresh(ctx context.Context) (T, error)

	// Functions returns the functions exposed by the provider that can be called
	// by other providers.
	Functions() ProviderFunctions
}

//provider "kubernetes" {
//	version = "1.0.0"
//	config {
//		type = environment.container_engine
//	}
//}
//
//// environments file
//enironment "development" {
//	container_engine = "docker"
//
//	providers {
//		kubernetes = {
//			version = ">=1.0.0"
//			source = "docker/kubernetes"
//		}
//	}
//}
//
//enironment "production" {
//	container_engine = "fargate"
//	providers {
//		kubernetes = {
//			version = "=1.0.0"
//			source = "aws/kubernetes"
//		}
//	}
//}
