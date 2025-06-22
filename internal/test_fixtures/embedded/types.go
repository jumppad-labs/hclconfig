package embedded

import "github.com/jumppad-labs/hclconfig/types"

// Container represents a container resource with embedded common fields
type Container struct {
	types.ResourceBase `hcl:",remain"`
	
	// Common container properties
	ID               string            `hcl:"id,optional"`
	Entrypoint       []string          `hcl:"entrypoint,optional"`
	Command          []string          `hcl:"command,optional"`
	Env              map[string]string `hcl:"env,optional"`
	DNS              []string          `hcl:"dns,optional"`
	Privileged       bool             `hcl:"privileged,optional"`
	MaxRestartCount  int              `hcl:"max_restart_count,optional"`
	
	// Specific container properties
	ContainerID      string           `hcl:"container_id,optional"`
}

func (c *Container) Process() error {
	return nil
}

// Sidecar represents a sidecar resource with embedded common fields
type Sidecar struct {
	types.ResourceBase `hcl:",remain"`
	
	// Common container properties
	ID               string            `hcl:"id,optional"`
	Entrypoint       []string          `hcl:"entrypoint,optional"`
	Command          []string          `hcl:"command,optional"`
	Env              map[string]string `hcl:"env,optional"`
	DNS              []string          `hcl:"dns,optional"`
	Privileged       bool             `hcl:"privileged,optional"`
	MaxRestartCount  int              `hcl:"max_restart_count,optional"`
	
	// Specific sidecar properties
	SidecarID        string           `hcl:"sidecar_id,optional"`
}

func (s *Sidecar) Process() error {
	return nil
}