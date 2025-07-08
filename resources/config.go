package resources

import "github.com/jumppad-labs/hclconfig/types"

const TypeConfig = "config"

type Config struct {
	types.ResourceBase `hcl:",remain"`

	Version string `hcl:"version,optional"`
}
