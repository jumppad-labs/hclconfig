package types

// Findable defines an interface used for locating resources
type Findable interface {
	FindResource(path string) (any, error)
	FindRelativeResource(path string, parentModule string) (any, error)
	FindResourcesByType(t string) ([]any, error)
	FindModuleResources(module string, includeSubModules bool) ([]any, error)
}
