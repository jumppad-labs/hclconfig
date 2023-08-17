package structs

import "github.com/jumppad-labs/hclconfig/types"

// TypeTemplate is the resource string for a Template resource
const TypeTestBrowser = "TestBrowser"

// Template allows the process of user defined templates
type TestBrowser struct {
	types.ResourceMetadata `hcl:",remain"`

	Config map[string]string `hcl:"config,optional" json:"config,omitempty"`
}
