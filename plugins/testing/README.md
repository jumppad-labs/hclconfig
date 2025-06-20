# Plugin Testing Package

This package provides test helpers for HCLConfig plugins, enabling fast and consistent testing of both in-process and external plugins.

## Overview

The testing package separates testing utilities from production code and provides:

- **In-process plugin testing** for fast development cycles
- **External plugin testing** for integration testing
- **Standardized test operations** for consistent coverage
- **Automatic cleanup** and resource management

## Basic Usage

```go
import (
    "testing"
    plugintesting "github.com/jumppad-labs/hclconfig/plugins/testing"
)

func TestMyPlugin(t *testing.T) {
    // Setup
    ph := plugintesting.InProcessPluginSetup(t, &MyPlugin{})
    ops := plugintesting.NewTestPluginOperations(ph)

    // Test
    ops.AssertSchemaValidation(1, "resource", "mytype")
    ops.TestCRUDOperations("resource", "mytype", testData)
}
```

## Key Functions

### Setup
- `InProcessPluginSetup(t, plugin)` - Fast in-process testing
- `ExternalPluginSetup(t, binaryPath)` - External process testing

### Operations
- `ops.AssertSchemaValidation()` - Schema validation
- `ops.TestCRUDOperations()` - Full CRUD testing
- `ops.TestBasicOperations()` - Basic operations only
- `ops.TestInvalidValidation()` - Error handling

### HCL Parsing
- `ParseHCLFile[T]()` - Low-level HCL parsing with explicit schema
- `ParseHCLWithPluginSchema[T]()` - Simple HCL parsing using plugin schema

### Helpers
- `NewPersonTestData()` - Pre-configured Person test data

## Benefits

1. **Fast**: In-process plugins eliminate startup overhead
2. **Consistent**: Standardized test patterns across all plugins
3. **Clean**: Separated from production code
4. **Comprehensive**: Covers schema, CRUD, and error cases
5. **Maintainable**: Centralized testing logic

See the main README for detailed examples and complete API documentation.