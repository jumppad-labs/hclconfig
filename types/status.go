package types

// Resource statuses recorded in Meta.Status. These are the only values xcl sets.
const (
	// StatusCreated is set when a provider has created the resource.
	StatusCreated = "created"

	// StatusUpdated is set when a provider has updated the resource.
	StatusUpdated = "updated"

	// StatusFailed is set when a provider call for the resource failed.
	// The resource is destroyed and created again on the next apply.
	StatusFailed = "failed"

	// StatusDestroyed is set when a provider has destroyed the resource.
	StatusDestroyed = "destroyed"

	// StatusDestroyFailed is set when destroying the resource failed.
	// The destroy is tried again on the next apply.
	StatusDestroyFailed = "destroy_failed"
)
