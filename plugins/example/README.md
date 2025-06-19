# HCL Plugin System Example

This example demonstrates how to create and use plugins with the HCL configuration system. It includes a complete implementation of a Person resource plugin with provider, lifecycle management, and testing.

## Files Overview

### Core Implementation
- **`person.go`** - Defines the Person resource struct
- **`provider.go`** - Implements the ExampleProvider for Person resources
- **`main.go`** - Demonstrates basic plugin usage and testing
- **`plugin_example.go`** - Shows how to create complete plugin implementations

### Test Fixtures
- **`test_fixtures/`** - Contains HCL files for testing
  - `simple_person.hcl` - Basic single person resource
  - `person.hcl` - Multiple person resources with different configurations
  - `complex_person.hcl` - Advanced test cases with edge cases

## Quick Start

### 1. Basic Plugin Usage

```go
// Create a plugin base
plugin := &plugins.PluginBase{}
plugin.SetLogger(&ExampleLogger{})

// Register a resource type
err := plugins.RegisterResourceProvider(
    plugin,
    "resource",           // type name
    "person",            // sub type name
    &example.Person{},   // resource instance
    &example.ExampleProvider{}, // provider implementation
)

// Use the plugin
err = plugin.Create("resource", "person", hclData)
```

### 2. Complete Plugin Implementation

```go
type PersonPlugin struct {
    plugins.PluginBase
}

func (p *PersonPlugin) Init() error {
    return plugins.RegisterResourceProvider(
        &p.PluginBase,
        "resource", "person",
        &example.Person{},
        &example.ExampleProvider{},
    )
}
```

## Running the Example

```bash
# From the example directory
go run *.go
```

This will:
1. Register the Person resource provider
2. Test all lifecycle operations (validate, create, destroy, etc.)
3. Demonstrate error handling
4. Show plugin initialization patterns

## Architecture Overview

### Resource Definition (`person.go`)
```go
type Person struct {
    types.ResourceBase `hcl:",remain"`
    FirstName string `hcl:"first_name" json:"first_name"`
    LastName  string `hcl:"last_name" json:"last_name"`
    Age       int    `hcl:"age,optional" json:"age,omitempty"`
    Email     string `hcl:"email,optional" json:"email,omitempty"`
    Address   string `hcl:"address,optional" json:"address,omitempty"`
}
```

### Provider Implementation (`provider.go`)
```go
type ExampleProvider struct {
    // Implements plugins.ResourceProvider[*Person]
}

func (p *ExampleProvider) Create(ctx context.Context, person *Person) (*Person, error) {
    // Implementation for creating person resources
}
// ... other lifecycle methods
```

### Plugin Registration
The system uses an adapter pattern to bridge between:
- **Typed providers** (`ResourceProvider[*Person]`) - Type-safe, strongly-typed
- **Plugin system** - Works with `[]byte` data and string type identifiers

## Key Concepts

### 1. Type Safety
- Resources must implement `types.Resource`
- Providers must implement `ResourceProvider[T]` for their resource type
- Compile-time type checking prevents mismatched types

### 2. Adapter Pattern
- `TypedProviderAdapter` converts between `[]byte` and concrete types
- Allows plugin system to work uniformly with all provider types
- Maintains type safety within provider implementations

### 3. Lifecycle Management
All resources support standard lifecycle operations:
- **Validate** - Check resource configuration
- **Create** - Create new resources
- **Destroy** - Clean up resources
- **Refresh** - Update resource state
- **Changed** - Detect configuration drift

## HCL Configuration Format

Resources are defined in HCL like this:

```hcl
resource "person" "john_doe" {
  first_name = "John"
  last_name  = "Doe"
  age        = 30
  email      = "john.doe@example.com"
  address    = "123 Main St, Anytown, USA"
}
```

## Extending the Example

### Adding New Resource Types

1. **Define the resource struct**:
```go
type Company struct {
    types.ResourceBase `hcl:",remain"`
    Name     string `hcl:"name"`
    Industry string `hcl:"industry"`
    Size     int    `hcl:"size,optional"`
}
```

2. **Implement the provider**:
```go
type CompanyProvider struct {
    // Implement plugins.ResourceProvider[*Company]
}
```

3. **Register the type**:
```go
err := plugins.RegisterResourceProvider(
    plugin, "resource", "company",
    &Company{}, &CompanyProvider{},
)
```

### Adding Provider Functions

Providers can expose functions that other providers can call:

```go
func (p *ExampleProvider) Functions() plugins.ProviderFunctions {
    return map[string]any{
        "validate_email": p.validateEmail,
        "format_name":   p.formatName,
    }
}
```

## Testing

The example includes comprehensive testing scenarios:

- **Happy path** - Normal resource operations
- **Error handling** - Invalid types, missing data
- **Edge cases** - Special characters, boundary values
- **Lifecycle completeness** - All CRUD operations

Run the tests by executing the main example program, which will automatically test all scenarios and report results.

## Integration with HCLConfig

In a real system, this plugin would be:
1. **Discovered** by the HCLConfig framework
2. **Initialized** during system startup
3. **Invoked** when processing HCL files containing person resources
4. **Managed** through the complete resource lifecycle

The plugin system provides the foundation for extensible, type-safe resource management in HCL-based configuration systems.