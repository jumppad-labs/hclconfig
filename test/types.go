package test

import (
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)

// Scenario represents a test scenario
type Scenario struct {
	types.ResourceMetadata `hcl:",remain" json:"resource_metadata"`

	// Description of what is being tested in this scenario
	Description string `hcl:"description,optional" json:"description,omitempty"`

	// Source is the location of the configuration that will be tested
	Source string `hcl:"source" json:"source"`

	// Contexts contain configuration parameters for each pass of the
	// parser.
	// If no context is specified the test will execute a single time
	// with default configuration
	Contexts []Context `hcl:"context,block" json:"contexts"`

	// Test configurations for the tests that will be executed
	Test []Test `hcl:"tests" json:"tests"`
}

// Context holds configuration prameters
type Context struct {
	// Description of what the context does
	Description string `hcl:"description,label" json:"description,omitempty"`

	// Env contains a map of environments that are set before each parse
	Env map[string]string `hcl:"env,optional" json:"env,omitempty"`

	// Variables are HCL variables that are set before each parse
	Variables cty.Value `hcl:"variables,optional" json:"variables,omitempty"`
}

// Test defines an individual test for the scenario
type Test struct {
	types.ResourceMetadata `hcl:",remain" json:"resource_metadata"`

	// Before contains commands which are executed before the Its blocks
	// they can be used to setup the test
	Before []FunctionDetails `hcl:"before,optional" json:"before,omitempty"`

	// Description of what the test is doing
	Description string `hcl:"description,optional" json:"description,omitempty"`

	// Its are the test steps, every test must have a minimum of one
	// It block
	Its []It `hcl:"it,block" json:"its"`

	// After contains commands which are executed after the Its blocks
	// they can be used to tear down the test
	After []FunctionDetails `hcl:"after,optional" json:"after,omitempty"`
}

// It defines a test step which comprises execution and assertions
type It struct {
	// Description of what the test is doing
	Description string `hcl:"description,label" json:"description,omitempty"`

	// Expect executes any command and parameter functions
	Expect []FunctionDetails `hcl:"expect" json:"expectations"`

	// To executes any assertion or comparitor functions
	To []FunctionDetails `hcl:"to,optional" json:"to,omitempty"`

	// Outputs are variables that are made available to other it blocks
	Outputs map[string]FunctionDetails `hcl:"outputs,optional" json:"outputs,omitempty"`
}

type FunctionDetails struct {
	Name        string      `hcl:"name" json:"name"`               // http_post
	Description string      `hcl:"description" json:"description"` // a http post is executed
	Type        string      `hcl:"type" json:"type"`               // command, parameter, comparitor
	Parameters  []cty.Value `hcl:"parameters" json:"parameters"`   // input params that function gets
}
