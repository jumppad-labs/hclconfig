package types

// Findable defines an interface used for locating resources
type Findable interface {
	FindResource(path string) (Resource, error)
	FindRelativeResource(path string, parentModule string) (Resource, error)
	FindResourcesByType(t string) ([]Resource, error)
	FindModuleResources(module string, includeSubModules bool) ([]Resource, error)
}
