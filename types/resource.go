package types

// Status defines the current state of a resource
type Status string

// PendingCreation means the resource has not yet been created
// it will be created on the next run
const PendingProcess Status = "pending_process"

// Applied means the resource has been successfully created
const Processed Status = "processed"

// Failed means the resource failed during creation
// if the action is Apply the resource will be re-created at the next run
const ProcessingFailed Status = "failed"

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
// it deinfes common meta data that all resources share
type ResourceMetadata struct {
	// Name is the name of the resource
	Name string `json:"name"`

	// Type is the type of resource, this is the text representation of the golang type
	Type string `json:"type"`

	// Module is the name of the module if a resource has been loaded from a module
	Module string `json:"module,omitempty"`

	// Status is the current status of the resource, this is always PendingCreation initially
	Status Status `json:"status,omitempty"`

	// Linked resources which must be set before this config can be processed
	ResourceLinks []string `json:"resource_links,omitempty" mapstructure:"resource_links"`

	// DependsOn is a user configurable list of dependencies for this resource
	DependsOn []string `json:"depends_on,omitempty" mapstructure:"depends_on"`

	// Enabled determines if a resource is enabled and should be processed
	Disabled bool `hcl:"disabled,optional" json:"disabled,omitempty"`
}

func (r *ResourceMetadata) Metadata() *ResourceMetadata {
	return r
}

//
// Body    *hclsyntax.Body
// Context *hcl.EvalContext
