package types

var TypeResource = "resource"

// Processable defines an optional interface that allows a resource to define a callback
// that is executed when the resources is processed by the DAG.
type Processable interface {
	// Process is called by the parser when the DAG is resolved
	Process() error
}

// Resource is an interface that all
type Resource interface {
	// return the resource Metadata
	Metadata() *ResourceMetadata
}

// ResourceMetadata is the embedded type for any config resources
// it defines common meta data that all resources share
type ResourceMetadata struct {
	// ID is the unique id for the resource
	// this follows the convention module_name.resource_name
	// i.e module.module1.module2.resource.container.mine
	ID string `json:"id"`

	// Name is the name of the resource
	// this is an internal property that is set from the stanza label
	Name string `json:"name"`

	// Type is the type of resource, this is the text representation of the golang type
	// this is an internal property that can not be set with hcl
	Type string `json:"type"`

	// Module is the name of the module if a resource has been loaded from a module
	// this is an internal property that can not be set with hcl
	Module string `json:"module,omitempty"`

	// Linked resources which must be set before this config can be processed
	// this is an internal property that can not be set with hcl
	ResourceLinks []string `json:"resource_links,omitempty"`

	// File is the absolute path of the file where the resource is defined
	// this is an internal property that can not be set with hcl
	File string `json:"file"`

	// ParentConfig allows the location of other resources in the config
	// this is an internal property that can not be set with hcl
	ParentConfig Findable `json:"-"`

	// Properties holds a collection that can be used to store adhoc data
	Properties map[string]interface{} `json:"properties,omitempty"`

	// # User configurable properties

	// DependsOn is a user configurable list of dependencies for this resource
	DependsOn []string `hcl:"depends_on,optional" json:"depends_on,omitempty"`

	// Enabled determines if a resource is enabled and should be processed
	Disabled bool `hcl:"disabled,optional" json:"disabled,omitempty"`
}

// Metadata is a function that ensures the struct that embeds the ResourceMetadata
// struct conforms to the interface Resource
func (r *ResourceMetadata) Metadata() *ResourceMetadata {
	return r
}
