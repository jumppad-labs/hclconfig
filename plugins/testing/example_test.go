package testing

import (
	"testing"
)

// This file demonstrates how easy it is to test new plugins using the test helpers

// DemoPluginTest shows how to test a new plugin
func DemoPluginTest() {
	// Example usage (would be in your plugin's test file):
	//
	// import (
	//     "testing"
	//     plugintesting "github.com/jumppad-labs/hclconfig/plugins/testing"
	// )
	//
	// func TestMyNewPlugin(t *testing.T) {
	//     // Setup in-process plugin for fast testing
	//     ph := plugintesting.InProcessPluginSetup(t, &MyNewPlugin{})
	//     ops := plugintesting.NewTestPluginOperations(ph)
	//
	//     // Test schema validation
	//     ops.AssertSchemaValidation(1, "resource", "myresource")
	//
	//     // Create test data
	//     testData := MyResourceTestData{
	//         Name: "test",
	//         Value: "example",
	//         Meta: struct {
	//             ID   string `json:"id"`
	//             Name string `json:"name"`
	//         }{
	//             ID:   "test.myresource.test_item",
	//             Name: "test_item",
	//         },
	//     }
	//
	//     // Test all CRUD operations
	//     ops.TestCRUDOperations("resource", "myresource", testData)
	//
	//     // Test HCL file parsing
	//     resources := ParseHCLWithPluginSchema(t, ph, "./testdata/myresource.hcl", MyResource{})
	//     require.Len(t, resources, 2)
	//     require.Equal(t, "expected", resources[0].Name)
	//
	//     // Test invalid validation
	//     ops.TestInvalidValidation("resource", "myresource")
	// }
	//
	// // For external plugin testing:
	// func TestMyNewPluginExternal(t *testing.T) {
	//     ph := plugintesting.ExternalPluginSetup(t, "./build/myplugin")
	//     ops := plugintesting.NewTestPluginOperations(ph)
	//     
	//     ops.AssertSchemaValidation(1, "resource", "myresource")
	//     ops.TestBasicOperations("resource", "myresource", testData)
	// }
}

// Example test data structure for a hypothetical plugin
type ExampleResourceTestData struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled,omitempty"`
	Meta    struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"meta"`
}

// TestHelperFunctions demonstrates the available helper functions
func TestHelperFunctions(t *testing.T) {
	t.Skip("This is documentation only")

	// Available setup functions:
	_ = InProcessPluginSetup   // func(t *testing.T, plugin plugins.Plugin) *TestPluginHost
	_ = ExternalPluginSetup   // func(t *testing.T, binaryPath string) *TestPluginHost

	// Available test operations:
	// ops := NewTestPluginOperations(host)
	// ops.AssertSchemaValidation(expectedCount, entityType, entitySubType)
	// ops.TestCRUDOperations(entityType, entitySubType, testData)
	// ops.TestInvalidValidation(entityType, entitySubType)
	// ops.TestBasicOperations(entityType, entitySubType, testData) // for problematic external plugins

	// Available parsing helpers:
	// ParseHCLWithPluginSchema[T any](t *testing.T, host *TestPluginHost, hclFilePath string, targetType T) []T
	// ParseHCLWithPluginSchemaToEntityData[T any](t *testing.T, host *TestPluginHost, hclFilePath string, targetType T) [][]byte
}