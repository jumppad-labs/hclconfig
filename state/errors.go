package state

import (
	"fmt"
	"strings"
)

// ResourceNotFoundError is returned when a resource cannot be found
type ResourceNotFoundError struct {
	Resource string
}

func (r ResourceNotFoundError) Error() string {
	return "resource not found: " + r.Resource
}

// ResourceExistsError is returned when trying to add a duplicate resource
type ResourceExistsError struct {
	Name string
}

func (r ResourceExistsError) Error() string {
	return "resource already exists: " + r.Name
}

// UnknownTypesError is returned when saved state holds resources whose types
// the registry can not create, usually because their plugin or type has not
// been registered. Loading fails rather than dropping those resources, so they
// are never erased from the saved state.
type UnknownTypesError struct {
	// Types is the sorted, unique list of unknown resource types
	Types []string
}

func (u UnknownTypesError) Error() string {
	return fmt.Sprintf(
		"saved state holds resources of unknown types: %s (register their types or plugins before loading state)",
		strings.Join(u.Types, ", "),
	)
}
