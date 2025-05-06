package structs

import (
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)

// TypeContainer is the resource string for a Container resource
const TypeContainer = "container"

// Container defines a structure for creating Docker containers
type Container struct {
	// embedded type holding name, etc
	types.ResourceBase `hcl:"rm,remain"`

	Default string `hcl:"default,optional" json:"default,omitempty" default:"hello world"` // A default value

	Networks   []NetworkAttachment `hcl:"network,block" json:"networks,omitempty"`         // Attach to the correct network // only when Image is specified
	NetworkObj Network             `hcl:"networkobj,optional" json:"networkobj,omitempty"` // Reference to an object

	Build      *Build            `hcl:"build,block" json:"build"`                        // Enables containers to be built on the fly
	Entrypoint []string          `hcl:"entrypoint,optional" json:"entrypoint,omitempty"` // entrypoint to use when starting the container
	Command    []string          `hcl:"command,optional" json:"command,omitempty"`       // command to use when starting the container
	Env        map[string]string `hcl:"env,optional" json:"env,omitempty"`               // environment variables to set when starting the container
	Volumes    []Volume          `hcl:"volume,block" json:"volumes,omitempty"`           // volumes to attach to the container
	Ports      []Port            `hcl:"port,block" json:"port,omitempty"`
	DNS        []string          `hcl:"dns,optional" json:"dns,omitempty"` // Add custom DNS servers to the container

	Privileged bool `hcl:"privileged,optional" json:"privileged,omitempty"` // run the container in privileged mode?

	// resource constraints
	Resources *Resources `hcl:"resources,block" json:"resources,omitempty"` // resource constraints for the container

	MaxRestartCount int `hcl:"max_restart_count,optional" json:"max_restart_count,omitempty" mapstructure:"max_restart_count"`

	// User block for mapping the user id and group id inside the container
	RunAs *User `hcl:"run_as,block" json:"run_as,omitempty" mapstructure:"run_as"`

	// output
	CreatedNetworks    []NetworkAttachment `hcl:"created_network,optional" json:"created_networks,omitempty"`         // Attach to the correct network // only when Image is specified
	CreatedNetworksMap map[string]Network  `hcl:"created_network_map,optional" json:"created_networks_map,omitempty"` // Attach to the correct network // only when Image is specified

	Output cty.Value `hcl:"output,optional" json:"output,omitempty"`
}

type User struct {
	// Username or UserID of the user to run the container as
	User string `hcl:"user" json:"user,omitempty" mapstructure:"user"`
	// Groupname GroupID of the user to run the container as
	Group string `hcl:"group" json:"group,omitempty" mapstructure:"group"`
}

type NetworkAttachment struct {
	ID        int      `hcl:"id,optional" json:"id,omitempty"`
	Name      string   `hcl:"name" json:"name"`
	IPAddress string   `hcl:"ip_address,optional" json:"ip_address,omitempty" mapstructure:"ip_address"`
	Aliases   []string `hcl:"aliases,optional" json:"aliases,omitempty"` // Network aliases for the resource
}

// Resources allows the setting of resource constraints for the Container
type Resources struct {
	CPU    int    `hcl:"cpu,optional" json:"cpu,omitempty"`                                // cpu limit for the container where 1 CPU = 1000
	CPUPin []int  `hcl:"cpu_pin,optional" json:"cpu_pin,omitempty" mapstructure:"cpu_pin"` // pin the container to one or more cpu cores
	Memory int    `hcl:"memory,optional" json:"memory,omitempty"`                          // max memory the container can consume in MB
	User   string `hcl:"user,optional" json:"user,omitempty"`
}

type Port struct {
	Local  int `hcl:"local" json:"local"`   // source path on the local machine for the volume
	Remote int `hcl:"remote" json:"remote"` // path to mount the volume inside the container
}

// Volume defines a folder, Docker volume, or temp folder to mount to the Container
type Volume struct {
	Source                      string `hcl:"source" json:"source"`                                                                                                                  // source path on the local machine for the volume
	Destination                 string `hcl:"destination" json:"destination"`                                                                                                        // path to mount the volume inside the container
	Type                        string `hcl:"type,optional" json:"type,omitempty"`                                                                                                   // type of the volume to mount [bind, volume, tmpfs]
	ReadOnly                    bool   `hcl:"read_only,optional" json:"read_only,omitempty" mapstructure:"read_only"`                                                                // specify that the volume is mounted read only
	BindPropagation             string `hcl:"bind_propagation,optional" json:"bind_propagation,omitempty" mapstructure:"bind_propagation"`                                           // propagation mode for bind mounts [shared, private, slave, rslave, rprivate]
	BindPropagationNonRecursive bool   `hcl:"bind_propagation_non_recursive,optional" json:"bind_propagation_non_recursive,omitempty" mapstructure:"bind_propagation_non_recursive"` // recursive bind mount, default true
}

// KV is a key/value type
type KV struct {
	Key   string `hcl:"key" json:"key"`
	Value string `hcl:"value" json:"value"`
}

// Build allows you to define the conditions for building a container
// on run from a Dockerfile
type Build struct {
	File    string `hcl:"file,optional" json:"file,omitempty"` // Location of build file inside build context defaults to ./Dockerfile
	Context string `hcl:"context" json:"context"`              // Path to build context
	Tag     string `hcl:"tag,optional" json:"tag,omitempty"`   // Image tag, defaults to latest
}

// Called when resource is read from the file, can be used to validate resource but
// you can not set any resource properties
// here as they are overwritten when the resource is processed by the dag
// ResourceBase properties can be set
func (c *Container) Parse(conf types.Findable) error {
	c.Meta.Properties["status"] = "something"
	return nil
}

// Called when resources is processed by the Graph
func (c *Container) Process() error {
	c.CreatedNetworks = []NetworkAttachment{
		NetworkAttachment{
			Name: "test1",
		},
		NetworkAttachment{
			Name: "test2",
		},
	}

	//c.CreatedNetworksMap = map[string]Network{
	//	"one": Network{ResourceBase: types.ResourceBase{ID: "one", Name: "test1"}},
	//	"two": Network{ResourceBase: types.ResourceBase{ID: "two", Name: "test2"}},
	//}

	return nil
}
