package plugins

import (
	"context"
)

// ProviderFunction represents a callable function exposed by a provider.
type ProviderFunction interface {
	// Call executes the provider function with the given parameters.
	// The param argument contains JSON-encoded function parameters.
	// It returns JSON-encoded results or an error if the function fails.
	Call(ctx context.Context, param []byte) ([]byte, error)
	// Schema returns the JSON schema for the function parameters.
	Schema() ([]byte, error)
}

// ProviderFunctions manages a collection of provider functions.
type ProviderFunctions interface {
	// Get retrieves a provider function by name.
	// It returns an error if the function does not exist.
	Get(name string) (ProviderFunction, error)

	// List returns all available provider functions with the given pattern.
	// The pattern can include regex, e.g., "kubernetes.*" to match all Kubernetes-related functions.
	Find(pattern string) (map[string]ProviderFunction, error)

	// List returns all available provider functions.
	List() (map[string]ProviderFunction, error)
}
