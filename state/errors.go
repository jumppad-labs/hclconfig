package state

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
