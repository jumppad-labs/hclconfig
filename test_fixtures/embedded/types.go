package embedded

import "github.com/instruqt/hclconfig/types"

type ContainerBase struct {
	// embedded type holding name, etc
	types.ResourceBase `hcl:",remain"`

	ID         string            `hcl:"id,optional" json:"id,omitempty"`
	Entrypoint []string          `hcl:"entrypoint,optional" json:"entrypoint,omitempty"` // entrypoint to use when starting the container
	Command    []string          `hcl:"command,optional" json:"command,omitempty"`       // command to use when starting the container
	Env        map[string]string `hcl:"env,optional" json:"env,omitempty"`               // environment variables to set when starting the container
	DNS        []string          `hcl:"dns,optional" json:"dns,omitempty"`               // Add custom DNS servers to the container
	Volumes    []Volume          `hcl:"volume,block" json:"volumes,omitempty"`           // volumes to attach to the container

	Privileged bool `hcl:"privileged,optional" json:"privileged,omitempty"` // run the container in privileged mode?

	MaxRestartCount int `hcl:"max_restart_count,optional" json:"max_restart_count,omitempty" mapstructure:"max_restart_count"`
}

type Volume struct {
	Source                      string `hcl:"source" json:"source"`                                                                                                                  // source path on the local machine for the volume
	Destination                 string `hcl:"destination" json:"destination"`                                                                                                        // path to mount the volume inside the container
	Type                        string `hcl:"type,optional" json:"type,omitempty"`                                                                                                   // type of the volume to mount [bind, volume, tmpfs]
	ReadOnly                    bool   `hcl:"read_only,optional" json:"read_only,omitempty" mapstructure:"read_only"`                                                                // specify that the volume is mounted read only
	BindPropagation             string `hcl:"bind_propagation,optional" json:"bind_propagation,omitempty" mapstructure:"bind_propagation"`                                           // propagation mode for bind mounts [shared, private, slave, rslave, rprivate]
	BindPropagationNonRecursive bool   `hcl:"bind_propagation_non_recursive,optional" json:"bind_propagation_non_recursive,omitempty" mapstructure:"bind_propagation_non_recursive"` // recursive bind mount, default true
}

const TypeContainer = "container"

type Container struct {
	ContainerBase `hcl:",remain"`

	ContainerID string `hcl:"container_id,optional" json:"container_id,omitempty"`
}

const TypeSidecar = "sidecar"

type Sidecar struct {
	ContainerBase `hcl:",remain"`

	SidecarID string `hcl:"sidecar_id,optional" json:"sidecar_id,omitempty"`
}
