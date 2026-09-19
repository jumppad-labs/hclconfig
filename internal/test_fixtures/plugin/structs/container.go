package structs

import (
	"github.com/jumppad-labs/xcl/types"
)

// TypeContainer is the resource string for a Container resource
const TypeContainer = "container"
const TypeSidecar = "sidecar"

// Container defines a structure for creating Docker containers
type ContainerBase struct {
	// embedded type holding name, etc
	types.ResourceBase `xcl:"rm,remain"`

	Default string `xcl:"default,optional" json:"default,omitempty" default:"hello world"` // A default value

	Networks   []NetworkAttachment `xcl:"network,block" json:"networks,omitempty"`         // Attach to the correct network // only when Image is specified
	NetworkObj Network             `xcl:"networkobj,optional" json:"networkobj,omitempty"` // Reference to an object

	Build      *Build            `xcl:"build,block" json:"build"`                        // Enables containers to be built on the fly
	Entrypoint []string          `xcl:"entrypoint,optional" json:"entrypoint,omitempty"` // entrypoint to use when starting the container
	Command    []string          `xcl:"command,optional" json:"command,omitempty"`       // command to use when starting the container
	Env        map[string]string `xcl:"env,optional" json:"env,omitempty"`               // environment variables to set when starting the container
	Volumes    []Volume          `xcl:"volume,block" json:"volumes,omitempty"`           // volumes to attach to the container
	Ports      []Port            `xcl:"port,block" json:"port,omitempty"`
	DNS        []string          `xcl:"dns,optional" json:"dns,omitempty"` // Add custom DNS servers to the container

	Privileged bool `xcl:"privileged,optional" json:"privileged,omitempty"` // run the container in privileged mode?

	// resource constraints
	Resources *Resources `xcl:"resources,block" json:"resources,omitempty"` // resource constraints for the container

	MaxRestartCount int `xcl:"max_restart_count,optional" json:"max_restart_count,omitempty"`

	// User block for mapping the user id and group id inside the container
	RunAs *User `xcl:"run_as,block" json:"run_as,omitempty"`

	// output
	CreatedNetworks    []NetworkAttachment `xcl:"created_network,optional" json:"created_networks,omitempty"`         // Attach to the correct network // only when Image is specified
	CreatedNetworksMap map[string]Network  `xcl:"created_network_map,optional" json:"created_networks_map,omitempty"` // Attach to the correct network // only when Image is specified
}

type User struct {
	// Username or UserID of the user to run the container as
	User string `xcl:"user" json:"user,omitempty"`
	// Groupname GroupID of the user to run the container as
	Group string `xcl:"group" json:"group,omitempty"`
}

type NetworkAttachment struct {
	ID        int      `xcl:"id,optional,key" json:"id,omitempty"` // pairs saved and configured attachments
	Name      string   `xcl:"name" json:"name"`
	IPAddress string   `xcl:"ip_address,optional" json:"ip_address,omitempty"`
	Aliases   []string `xcl:"aliases,optional" json:"aliases,omitempty"` // Network aliases for the resource

	// AssignedAddress is set by the provider when the container is attached
	AssignedAddress string `xcl:"assigned_address,optional,computed" json:"assigned_address,omitempty"`
}

// Resources allows the setting of resource constraints for the Container
type Resources struct {
	CPU    int    `xcl:"cpu,optional" json:"cpu,omitempty"`         // cpu limit for the container where 1 CPU = 1000
	CPUPin []int  `xcl:"cpu_pin,optional" json:"cpu_pin,omitempty"` // pin the container to one or more cpu cores
	Memory int    `xcl:"memory,optional" json:"memory,omitempty"`   // max memory the container can consume in MB
	User   string `xcl:"user,optional" json:"user,omitempty"`
}

type Port struct {
	Local  int `xcl:"local" json:"local"`   // source path on the local machine for the volume
	Remote int `xcl:"remote" json:"remote"` // path to mount the volume inside the container
}

// Volume defines a folder, Docker volume, or temp folder to mount to the Container
type Volume struct {
	Source                      string `xcl:"source" json:"source"`                                                                    // source path on the local machine for the volume
	Destination                 string `xcl:"destination" json:"destination"`                                                          // path to mount the volume inside the container
	Type                        string `xcl:"type,optional" json:"type,omitempty"`                                                     // type of the volume to mount [bind, volume, tmpfs]
	ReadOnly                    bool   `xcl:"read_only,optional" json:"read_only,omitempty"`                                           // specify that the volume is mounted read only
	BindPropagation             string `xcl:"bind_propagation,optional" json:"bind_propagation,omitempty"`                             // propagation mode for bind mounts [shared, private, slave, rslave, rprivate]
	BindPropagationNonRecursive bool   `xcl:"bind_propagation_non_recursive,optional" json:"bind_propagation_non_recursive,omitempty"` // recursive bind mount, default true
}

// KV is a key/value type
type KV struct {
	Key   string `xcl:"key" json:"key"`
	Value string `xcl:"value" json:"value"`
}

// Build allows you to define the conditions for building a container
// on run from a Dockerfile
type Build struct {
	File    string `xcl:"file,optional" json:"file,omitempty"` // Location of build file inside build context defaults to ./Dockerfile
	Context string `xcl:"context" json:"context"`              // Path to build context
	Tag     string `xcl:"tag,optional" json:"tag,omitempty"`   // Image tag, defaults to latest

	// ImageID is set by the provider when the image is built
	ImageID string `xcl:"image_id,optional,computed" json:"image_id,omitempty"`
}

type Container struct {
	ContainerBase `xcl:",remain"`

	ContainerID string `xcl:"container_id,optional" json:"container_id,omitempty"`
}

type Sidecar struct {
	ContainerBase `xcl:",remain"`

	SidecarID string `xcl:"sidecar_id,optional" json:"sidecar_id,omitempty"`
}
