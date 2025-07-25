package testing

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/hclconfig/internal/schema"
	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/plugins/mocks"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// TestPluginHost provides a convenient interface for testing plugins
type TestPluginHost struct {
	plugins.PluginHost // Embed the interface, not a specific implementation
	t                  *testing.T
}

// InProcessPluginSetup creates an in-process plugin host for testing
func InProcessPluginSetup(t *testing.T, plugin plugins.Plugin) *TestPluginHost {
	logger := logger.NewTestLogger(t)
	state := mocks.NewMockState(t)

	ph, err := plugins.NewDirectPluginHost(logger, state, plugin)
	require.NoError(t, err, "In-process plugin should initialize without error")

	t.Cleanup(func() {
		ph.Stop()
	})

	return &TestPluginHost{
		PluginHost: ph,
		t:          t,
	}
}

// ExternalPluginSetup creates an external process plugin host for testing
func ExternalPluginSetup(t *testing.T, binaryPath string) *TestPluginHost {
	logger := logger.NewTestLogger(t)
	ph := plugins.NewGRPCPluginHost(logger, nil)

	err := ph.Start(binaryPath)
	require.NoError(t, err, "External plugin should start without error")

	t.Cleanup(func() {
		ph.Stop()
	})

	return &TestPluginHost{
		PluginHost: ph,
		t:          t,
	}
}

// TestPluginOperations provides common test operations for plugins
type TestPluginOperations struct {
	host *TestPluginHost
}

// NewTestPluginOperations creates a new test operations helper
func NewTestPluginOperations(host *TestPluginHost) *TestPluginOperations {
	return &TestPluginOperations{host: host}
}

// AssertSchemaValidation validates that the plugin returns expected schema
func (ops *TestPluginOperations) AssertSchemaValidation(expectedCount int, entityType, entitySubType string) {
	types := ops.host.GetTypes()
	require.Len(ops.host.t, types, expectedCount, "Should have expected number of registered types")

	if expectedCount > 0 {
		foundType := types[0]
		require.Equal(ops.host.t, entityType, foundType.Type, "Type should match expected")
		require.Equal(ops.host.t, entitySubType, foundType.SubType, "SubType should match expected")
		require.NotEmpty(ops.host.t, foundType.Schema, "Schema should not be empty")
	}
}

// TestValidate tests the Validate operation using HCL test data
func (ops *TestPluginOperations) TestValidate(entityType, entitySubType string, hclFilePath string) {
	// Parse HCL file to get test data
	types := ops.host.GetTypes()
	require.NotEmpty(ops.host.t, types, "Plugin should have registered types")

	result := ParseHCLFile(ops.host.t, hclFilePath, types[0].Schema, (*interface{})(nil))
	require.NotEmpty(ops.host.t, result.Objects, "Should have test data from HCL file")

	// Test each object from the HCL file
	for i, obj := range result.Objects {
		dataJSON, err := json.Marshal(obj)
		require.NoError(ops.host.t, err, "Should marshal test data to JSON for object %d", i)

		err = ops.host.Validate(entityType, entitySubType, dataJSON)
		require.NoError(ops.host.t, err, "Should validate object %d", i)
	}
}

// TestCreate tests the Create operation using HCL test data
func (ops *TestPluginOperations) TestCreate(entityType, entitySubType string, hclFilePath string) {
	// Parse HCL file to get test data
	types := ops.host.GetTypes()
	require.NotEmpty(ops.host.t, types, "Plugin should have registered types")

	result := ParseHCLFile(ops.host.t, hclFilePath, types[0].Schema, (*interface{})(nil))
	require.NotEmpty(ops.host.t, result.Objects, "Should have test data from HCL file")

	// Test creating each object from the HCL file
	for i, obj := range result.Objects {
		dataJSON, err := json.Marshal(obj)
		require.NoError(ops.host.t, err, "Should marshal test data to JSON for object %d", i)

		_, err = ops.host.Create(entityType, entitySubType, dataJSON)
		require.NoError(ops.host.t, err, "Should create object %d", i)
	}
}

// TestChanged tests the Changed operation using HCL test data
func (ops *TestPluginOperations) TestChanged(entityType, entitySubType string, hclFilePath string) {
	// Parse HCL file to get test data
	types := ops.host.GetTypes()
	require.NotEmpty(ops.host.t, types, "Plugin should have registered types")

	result := ParseHCLFile(ops.host.t, hclFilePath, types[0].Schema, (*interface{})(nil))
	require.NotEmpty(ops.host.t, result.Objects, "Should have test data from HCL file")

	// Test checking changes for each object from the HCL file
	for i, obj := range result.Objects {
		dataJSON, err := json.Marshal(obj)
		require.NoError(ops.host.t, err, "Should marshal test data to JSON for object %d", i)

		changed, err := ops.host.Changed(entityType, entitySubType, dataJSON, dataJSON)
		require.NoError(ops.host.t, err, "Should check for changes without error for object %d", i)
		require.False(ops.host.t, changed, "Newly created object %d should not be changed", i)
	}
}

// TestDestroy tests the Destroy operation using HCL test data
func (ops *TestPluginOperations) TestDestroy(entityType, entitySubType string, hclFilePath string) {
	// Parse HCL file to get test data
	types := ops.host.GetTypes()
	require.NotEmpty(ops.host.t, types, "Plugin should have registered types")

	result := ParseHCLFile(ops.host.t, hclFilePath, types[0].Schema, (*interface{})(nil))
	require.NotEmpty(ops.host.t, result.Objects, "Should have test data from HCL file")

	// Test destroying each object from the HCL file
	for i, obj := range result.Objects {
		dataJSON, err := json.Marshal(obj)
		require.NoError(ops.host.t, err, "Should marshal test data to JSON for object %d", i)

		err = ops.host.Destroy(entityType, entitySubType, dataJSON)
		require.NoError(ops.host.t, err, "Should destroy object %d", i)
	}
}

// TestCRUDOperations tests all CRUD operations with the provided test data (kept for backwards compatibility)
func (ops *TestPluginOperations) TestCRUDOperations(entityType, entitySubType string, testData interface{}) {
	// Marshal test data to JSON
	dataJSON, err := json.Marshal(testData)
	require.NoError(ops.host.t, err, "Should marshal test data to JSON")

	// Test Validate with valid data
	err = ops.host.Validate(entityType, entitySubType, dataJSON)
	require.NoError(ops.host.t, err, "Should validate valid data")

	// Test Create
	_, err = ops.host.Create(entityType, entitySubType, dataJSON)
	require.NoError(ops.host.t, err, "Should create resource successfully")

	// Test Changed
	changed, err := ops.host.Changed(entityType, entitySubType, dataJSON, dataJSON)
	require.NoError(ops.host.t, err, "Should check for changes without error")
	require.False(ops.host.t, changed, "Newly created resource should not be changed")

	// Test Destroy
	err = ops.host.Destroy(entityType, entitySubType, dataJSON)
	require.NoError(ops.host.t, err, "Should destroy resource successfully")
}

// TestInvalidValidation tests validation with invalid data
func (ops *TestPluginOperations) TestInvalidValidation(entityType, entitySubType string) {
	// Test with malformed JSON
	invalidJSON := []byte(`{"invalid": "data", missing quote}`)
	err := ops.host.Validate(entityType, entitySubType, invalidJSON)
	require.Error(ops.host.t, err, "Should fail validation with invalid data")
}

// TestBasicOperations tests basic create and validate operations (for external plugins with issues)
func (ops *TestPluginOperations) TestBasicOperations(entityType, entitySubType string, testData interface{}) {
	// Marshal test data to JSON
	dataJSON, err := json.Marshal(testData)
	require.NoError(ops.host.t, err, "Should marshal test data to JSON")

	// Test Validate
	err = ops.host.Validate(entityType, entitySubType, dataJSON)
	require.NoError(ops.host.t, err, "Should validate data")

	// Test Create (if this works, we know the basic plugin functionality is working)
	_, err = ops.host.Create(entityType, entitySubType, dataJSON)
	require.NoError(ops.host.t, err, "Should create resource")
}

// HCLParseResult represents the result of parsing an HCL file
type HCLParseResult[T any] struct {
	Objects []T
	Count   int
}

// ParseHCLFile parses an HCL file and returns an array of strongly-typed objects
// This function takes:
// - hclFilePath: path to the HCL file to parse
// - pluginSchema: the schema from the plugin to create the wire type
// - targetType: a zero-value instance of the target type (e.g., person.Person{})
// It returns a slice of the target type with all parsed objects
func ParseHCLFile[T any](t *testing.T, hclFilePath string, pluginSchema []byte, targetType T) HCLParseResult[T] {
	// Create wire type from plugin schema
	wireType, err := schema.CreateInstanceFromSchema(pluginSchema, nil)
	require.NoError(t, err, "Should be able to create struct from schema")

	// Parse HCL file
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(hclFilePath)
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags.Error())

	// Create eval context with some basic variables
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{},
	}

	// Add common test variables
	valMap := map[string]cty.Value{}
	valMap["a"] = cty.StringVal("foo")
	ctx.Variables["var"] = cty.ObjectVal(valMap)

	// Parse all blocks into objects
	var results []T
	body := f.Body.(*hclsyntax.Body)

	for _, block := range body.Blocks {
		// Decode the block into the wire type
		diags := gohcl.DecodeBody(block.Body, ctx, wireType)
		require.False(t, diags.HasErrors(), "Failed to decode HCL block: %s", diags.Error())

		// Create a new instance of the target type
		targetValue := reflect.New(reflect.TypeOf(targetType)).Interface()

		// Unmarshal from wire type to concrete type
		schema.UnmarshalUntyped(wireType, targetValue)

		// Dereference the pointer and append to results
		results = append(results, reflect.ValueOf(targetValue).Elem().Interface().(T))
	}

	return HCLParseResult[T]{
		Objects: results,
		Count:   len(results),
	}
}

// ParseHCLWithPluginSchema parses an HCL file using the plugin's schema and returns typed objects
// This is a simple helper that just returns the parsed objects for flexible testing
func ParseHCLWithPluginSchema[T any](t *testing.T, host *TestPluginHost, hclFilePath string, targetType T) []T {
	// Get the first registered type (assuming single type plugin for simplicity)
	types := host.GetTypes()
	require.NotEmpty(t, types, "Plugin should have registered types")

	// Parse the HCL file
	result := ParseHCLFile(t, hclFilePath, types[0].Schema, targetType)

	return result.Objects
}

// ParseHCLWithPluginSchemaToEntityData parses an HCL file using the plugin's schema and returns JSON-serialized entity data
// This is useful for tests that need to pass data directly to plugin methods that expect []byte
func ParseHCLWithPluginSchemaToEntityData[T any](t *testing.T, host *TestPluginHost, hclFilePath string, targetType T) [][]byte {
	// Parse HCL file into typed objects
	objects := ParseHCLWithPluginSchema(t, host, hclFilePath, targetType)

	// Convert each object to JSON bytes
	var entityData [][]byte
	for i, obj := range objects {
		jsonBytes, err := json.Marshal(obj)
		require.NoError(t, err, "Should marshal object %d to JSON", i)
		entityData = append(entityData, jsonBytes)
	}

	return entityData
}

// BuildPlugin builds a plugin binary for testing
// Returns an error if the build fails
func BuildPlugin(t *testing.T, pluginDir string) error {
	cmd := exec.Command("make", "build")
	cmd.Dir = pluginDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Plugin build failed: %s", string(output))
		return err
	}
	return nil
}
