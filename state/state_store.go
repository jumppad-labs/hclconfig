package state

//go:generate mockery --name StateStore --output ./mocks --outpkg mocks --filename mock_state_store.go

// StateStore provides persistence for configuration state across parser runs.
// It enables tracking of resource lifecycle (create, update, delete) by storing
// the previous configuration state and comparing it with the current state.
type StateStore interface {
	// Load retrieves the previously saved configuration state.
	// Returns nil if no state exists (first run).
	// Returns an error if the state exists but cannot be loaded.
	Load() (any, error)

	// Save persists the current configuration state.
	// The implementation should ensure atomic writes to prevent corruption.
	Save(config any) error

	// Exists returns true if a saved state exists.
	Exists() bool

	// Clear removes the saved state.
	// This is useful for resetting or during cleanup operations.
	Clear() error
}
