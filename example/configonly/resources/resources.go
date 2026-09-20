// Package resources holds the Go types for the blocks in the configuration
// only example, a small Kubernetes-like deployment. The names are the ones a
// Kubernetes user would recognise rather than a literal translation of a
// manifest: there is no apiVersion or kind, attribute names are snake_case,
// and blocks are linked by reference rather than by matching names by hand.
//
// A block type is a Go struct embedding types.ResourceBase, which carries the
// common block metadata. The xcl tags map configuration to fields, the json
// tags name the fields the same way when a resource is saved to state.
//
// Together these types show the three things configuration usually needs:
//
//   - nested blocks, i.e. resources inside container
//   - repeated blocks, held in a slice, i.e. several container or port blocks
//   - links between resources, i.e. the service reading a port from the
//     deployment
package resources

import "github.com/jumppad-labs/xcl/types"

// ConfigMap defines the block type `config_map`, a bag of values the
// deployment reads. A map attribute holds values whose names are not known
// ahead of time, and a single value can be read from it with
// `resource.config_map.api.data.db_host`.
type ConfigMap struct {
	types.ResourceBase `xcl:",remain"`

	Data map[string]string `xcl:"data" json:"data"`
}

// Deployment defines the block type `deployment`. It holds repeated container
// blocks, each of which nests further blocks of its own.
type Deployment struct {
	types.ResourceBase `xcl:",remain"`

	Replicas int `xcl:"replicas,optional" json:"replicas,omitempty"`

	// Containers is a repeated block, one entry per container block in the
	// configuration. A repeated block is a slice, a block that appears once is
	// a pointer.
	Containers []Container `xcl:"container,block" json:"container"`

	// Volumes are mounted by the containers above through their name
	Volumes []Volume `xcl:"volume,block" json:"volume,omitempty"`
}

// Container is the nested `container` block of a deployment. It is not a
// resource of its own, so it does not embed types.ResourceBase and can not be
// referenced by its own id, its fields are reached through the deployment
// that holds it, i.e.
// `resource.deployment.api.container[0].port[0].container_port`.
type Container struct {
	Name  string `xcl:"name" json:"name"`
	Image string `xcl:"image" json:"image"`

	Ports []Port   `xcl:"port,block" json:"port,omitempty"`
	Env   []EnvVar `xcl:"env,block" json:"env,omitempty"`

	// Resources appears at most once, so it is a pointer and is nil when the
	// container does not set it
	Resources *ResourceRequirements `xcl:"resources,block" json:"resources,omitempty"`

	VolumeMounts []VolumeMount `xcl:"volume_mount,block" json:"volume_mount,omitempty"`
}

// Port is a nested `port` block of a container
type Port struct {
	Name          string `xcl:"name" json:"name"`
	ContainerPort int    `xcl:"container_port" json:"container_port"`
}

// EnvVar is a nested `env` block of a container, its value is usually read
// from a config map
type EnvVar struct {
	Name  string `xcl:"name" json:"name"`
	Value string `xcl:"value" json:"value"`
}

// ResourceRequirements is the nested `resources` block of a container, it
// nests two blocks of its own, blocks can be nested as deeply as the
// configuration needs
type ResourceRequirements struct {
	Limits   *ResourceQuantities `xcl:"limits,block" json:"limits,omitempty"`
	Requests *ResourceQuantities `xcl:"requests,block" json:"requests,omitempty"`
}

// ResourceQuantities is the nested `limits` or `requests` block of a
// resources block, the same Go type serves both
type ResourceQuantities struct {
	CPU    string `xcl:"cpu,optional" json:"cpu,omitempty"`
	Memory string `xcl:"memory,optional" json:"memory,omitempty"`
}

// VolumeMount is a nested `volume_mount` block of a container, naming a
// volume the deployment declares
type VolumeMount struct {
	Name string `xcl:"name" json:"name"`
	Path string `xcl:"path" json:"path"`
}

// Volume is a nested `volume` block of a deployment, this one is always
// filled from a config map, which it names by id
type Volume struct {
	Name      string `xcl:"name" json:"name"`
	ConfigMap string `xcl:"config_map" json:"config_map"`
}

// Service defines the block type `service`, it routes traffic to the
// containers of a deployment.
//
// Kubernetes matches a service to its pods with a label selector because a
// manifest has no way to point at another object. Configuration parsed by xcl
// does, so the service names the deployment by its id and reads the port out
// of it, and the two can not drift apart.
type Service struct {
	types.ResourceBase `xcl:",remain"`

	Deployment string `xcl:"deployment" json:"deployment"`
	Port       int    `xcl:"port" json:"port"`
	TargetPort int    `xcl:"target_port" json:"target_port"`
}

// Ingress defines the block type `ingress`, it routes a host to a service
type Ingress struct {
	types.ResourceBase `xcl:",remain"`

	Host  string `xcl:"host" json:"host"`
	Rules []Rule `xcl:"rule,block" json:"rule"`
}

// Rule is a nested `rule` block of an ingress, sending one path to one
// service, named by its id
type Rule struct {
	Path    string `xcl:"path" json:"path"`
	Service string `xcl:"service" json:"service"`
	Port    int    `xcl:"port" json:"port"`
}
