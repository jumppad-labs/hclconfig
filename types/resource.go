package types

var TypeResource = "resource"

// Parsable defines an optional interface that allows a resource to be
// modified directly after it has been loaded from a file
//
// Parsable should be implemented when you want to do basic validation of
// resources before they are processed by the graph.
//
// Parse is called sequentially for each resource as it is loaded from the
// config file. This occurs before the graph of dependent resources has been
// built.
type Parsable interface {
	// Parse is called when the resource is created from a file, it is called
	// after all configuration files have been read a list of which are passed
	// to Parse to allow validation based on other resources.
	//
	// Note: it is not possible to set resource properties from parse
	// as all properties are overwritten when the resource is processed
	// by the dag and any dependencies are resolved.
	//
	// ResourceBase can be set by this method as this is not overridden
	// when processed.
	Parse(config Findable) error
}

// Resource is an interface that all
type Resource interface {
	// return the resource Metadata
	Metadata() *Meta
	GetDisabled() bool
	SetDisabled(bool)
	GetDependencies() []string
	SetDependencies([]string)
	AddDependency(string)
}

type Meta struct {
	// ID is the unique id for the resource
	// this follows the convention module_name.resource_name
	// i.e module.module1.module2.resource.container.mine
	ID string `hcl:"id,optional" json:"id"`

	// Name is the name of the resource
	// this is an internal property that is set from the stanza label
	Name string `hcl:"name,optional" json:"name"`

	// Type is the type of resource, this is the text representation of the golang type
	// this is an internal property that can not be set with hcl
	Type string `hcl:"type,optional" json:"type"`

	// Module is the name of the module if a resource has been loaded from a module
	// this is an internal property that can not be set with hcl
	Module string `hcl:"module,optional" json:"module,omitempty"`

	// File is the absolute path of the file where the resource is defined
	// this is an internal property that can not be set with hcl
	File string `hcl:"file,optional" json:"file"`

	// Line is the starting line number where the resource is located in the
	// file from where it was originally parsed
	Line int `hcl:"line,optional" json:"line"`

	// Column is the starting column number where the resource is located in the
	// file from where it was originally parsed
	Column int `hcl:"column,optional" json:"column"`

	// Properties holds a collection that can be used to store adhoc data
	Properties map[string]any `json:"properties,omitempty"`

	// Linked resources which must be set before this config can be processed
	// this is an internal property that can not be set with hcl
	Links []string `json:"links,omitempty"`
}

// ResourceBase is the embedded type for any config resources
// it defines common meta data that all resources share
type ResourceBase struct {
	// DependsOn is a user configurable list of dependencies for this resource
	DependsOn []string `hcl:"depends_on,optional" json:"depends_on,omitempty"`

	// Enabled determines if a resource is enabled and should be processed
	Disabled bool `hcl:"disabled,optional" json:"disabled,omitempty"`

	Meta Meta `hcl:"meta,optional" json:"meta,omitempty"`
}

// Metadata is a function that ensures the struct that embeds the ResourceBase
// struct conforms to the interface Resource
func (r *ResourceBase) Metadata() *Meta {
	return &r.Meta
}

func (r *ResourceBase) GetDisabled() bool {
	return r.Disabled
}

func (r *ResourceBase) SetDisabled(v bool) {
	r.Disabled = v
}

func (r *ResourceBase) GetDependencies() []string {
	return r.DependsOn
}

func (r *ResourceBase) SetDependencies(v []string) {
	r.DependsOn = v
}

func (r *ResourceBase) AddDependency(v string) {
	r.DependsOn = appendIfNotContains(r.DependsOn, v)
}

func appendIfNotContains(list []string, value string) []string {
	contains := false
	for _, item := range list {
		if value == item {
			contains = true
		}
	}

	if !contains {
		list = append(list, value)
	}
	return list
}
