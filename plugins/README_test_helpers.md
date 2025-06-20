# Plugin Test Helpers

The `plugins/testing` package provides convenient test helpers for testing HCLConfig plugins, making it easy to create comprehensive test suites for both in-process and external plugins.

## Quick Start

### In-Process Plugin Testing (Recommended)

```go
import (
    "testing"
    plugintesting "github.com/jumppad-labs/hclconfig/plugins/testing"
)

func TestMyPlugin(t *testing.T) {
    // Setup in-process plugin for fast testing
    ph := plugintesting.InProcessPluginSetup(t, &MyPlugin{})
    ops := plugintesting.NewTestPluginOperations(ph)

    // Test schema validation
    ops.AssertSchemaValidation(1, "resource", "myresource")

    // Create test data
    testData := MyResourceTestData{
        Name: "test",
        Value: "example",
    }

    // Test all CRUD operations
    ops.TestCRUDOperations("resource", "myresource", testData)

    // Test invalid validation
    ops.TestInvalidValidation("resource", "myresource")
}
```

### External Plugin Testing

```go
func TestMyPluginExternal(t *testing.T) {
    ph := plugintesting.ExternalPluginSetup(t, "./build/myplugin")
    ops := plugintesting.NewTestPluginOperations(ph)
    
    ops.AssertSchemaValidation(1, "resource", "myresource")
    
    // Use basic operations for external plugins that might have issues
    ops.TestBasicOperations("resource", "myresource", testData)
}
```

## Available Functions

### Setup Functions

- `plugintesting.InProcessPluginSetup(t *testing.T, plugin plugins.Plugin) *TestPluginHost`
  - Creates an in-process plugin host for fast testing
  - Automatically handles cleanup via t.Cleanup()

- `plugintesting.ExternalPluginSetup(t *testing.T, binaryPath string) *TestPluginHost`
  - Creates an external process plugin host
  - Automatically handles cleanup via t.Cleanup()

### Test Operations

- `ops.AssertSchemaValidation(expectedCount, entityType, entitySubType)`
  - Validates that the plugin returns the expected schema

- `ops.TestCRUDOperations(entityType, entitySubType, testData)`
  - Tests Create, Validate, Changed, and Destroy operations
  - Use this for full CRUD testing

- `ops.TestBasicOperations(entityType, entitySubType, testData)`
  - Tests just Validate and Create operations
  - Useful for external plugins with issues

- `ops.TestInvalidValidation(entityType, entitySubType)`
  - Tests validation with malformed JSON data

### Helper Functions

- `plugintesting.NewPersonTestData(firstName, lastName, id, name string, age int, email string) PersonTestData`
  - Creates properly formatted Person test data with metadata

### HCL Parsing Functions

- `plugintesting.ParseHCLFile[T](t *testing.T, hclFilePath string, pluginSchema []byte, targetType T) HCLParseResult[T]`
  - Low-level HCL parsing with explicit schema
  - Takes an HCL file path, plugin schema, and target type
  - Returns `HCLParseResult[T]` with parsed objects and count

- `plugintesting.ParseHCLWithPluginSchema[T](ops *TestPluginOperations, hclFilePath string, targetType T) []T`
  - Simple HCL parsing using plugin's schema
  - Automatically uses plugin schema from operations
  - Returns slice of strongly-typed objects

## Benefits

1. **Consistent Testing**: All plugins use the same test patterns
2. **Fast Development**: In-process plugins eliminate process startup overhead
3. **Easy Maintenance**: Centralized test logic reduces duplication
4. **Comprehensive Coverage**: Tests cover schema, parsing, and CRUD operations
5. **Backward Compatibility**: Supports both in-process and external plugins
6. **Clean Separation**: Testing utilities are separated from production code

## Example Test Structure

```go
import (
    "testing"
    plugintesting "github.com/jumppad-labs/hclconfig/plugins/testing"
)

// Test schema and basic functionality
func TestMyPluginSchema(t *testing.T) {
    ph := plugintesting.InProcessPluginSetup(t, &MyPlugin{})
    ops := plugintesting.NewTestPluginOperations(ph)
    ops.AssertSchemaValidation(1, "resource", "myresource")
}

// Test CRUD operations
func TestMyPluginCRUD(t *testing.T) {
    ph := plugintesting.InProcessPluginSetup(t, &MyPlugin{})
    ops := plugintesting.NewTestPluginOperations(ph)
    
    testData := createTestData()
    ops.TestCRUDOperations("resource", "myresource", testData)
}

// Test HCL file parsing
func TestMyPluginHCLParsing(t *testing.T) {
    ph := plugintesting.InProcessPluginSetup(t, &MyPlugin{})
    ops := plugintesting.NewTestPluginOperations(ph)
    
    // Parse HCL file into typed objects
    resources := plugintesting.ParseHCLWithPluginSchema(ops, "./testdata/myresource.hcl", MyResource{})
    
    // Test the results however you want
    require.Len(t, resources, 2)
    require.Equal(t, "expected_name", resources[0].Name)
    require.True(t, resources[1].Enabled)
}

// Test external plugin compatibility
func TestMyPluginExternal(t *testing.T) {
    ph := plugintesting.ExternalPluginSetup(t, "./build/myplugin")
    ops := plugintesting.NewTestPluginOperations(ph)
    ops.TestBasicOperations("resource", "myresource", testData)
}
```

This pattern ensures comprehensive testing while keeping test code clean and maintainable, with clear separation between production and testing code.