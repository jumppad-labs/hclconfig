package hclconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
)

// Test types for provider registry testing
type TestConfig struct {
	Field1 string `hcl:"field1,optional"`
	Field2 int    `hcl:"field2,optional"`
}

type TestResource struct {
	types.ResourceBase `hcl:",remain"`
	Value              string `hcl:"value"`
}

type TestProvider struct {
	initialized bool
	config      TestConfig
}

func (p *TestProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger plugins.Logger, config TestConfig) error {
	p.initialized = true
	p.config = config
	return nil
}

func (p *TestProvider) Create(ctx context.Context, resource *TestResource) (*TestResource, error) {
	return resource, nil
}

func (p *TestProvider) Destroy(ctx context.Context, resource *TestResource, force bool) error {
	return nil
}

func (p *TestProvider) Refresh(ctx context.Context, resource *TestResource) error {
	return nil
}

func (p *TestProvider) Changed(ctx context.Context, current *TestResource, desired *TestResource) (bool, error) {
	return false, nil
}

func (p *TestProvider) Update(ctx context.Context, resource *TestResource) error {
	return nil
}

func (p *TestProvider) Functions() plugins.ProviderFunctions {
	return nil
}

type RegistryTestPlugin struct {
	plugins.PluginBase
}


func (p *RegistryTestPlugin) Init(logger plugins.Logger, state plugins.State) error {
	// Register a test resource type to set up config type in PluginBase
	testResource := &TestResource{}
	testProvider := &TestProvider{}
	config := TestConfig{}
	
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"test",
		testResource,
		testProvider,
		config,
	)
}

func TestPluginRegistryRegisterProvider(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))

	// Create a test provider
	provider := &resources.Provider{
		Source:  "test/provider",
		Version: "1.0.0",
	}
	provider.Metadata().Name = "test-provider"
	provider.Metadata().Type = resources.TypeProvider

	// Register a test plugin
	plugin := &RegistryTestPlugin{}
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	registry.RegisterPluginSource("test/provider", plugin)

	// Register the provider
	ctx := &hcl.EvalContext{}
	err = registry.RegisterProvider(provider, ctx)
	require.NoError(t, err)

	// Verify provider was registered
	providers := registry.ListProviders()
	require.Contains(t, providers, "test-provider")

	// Get the provider
	providerConfig, err := registry.GetProvider("test-provider")
	require.NoError(t, err)
	require.Equal(t, "test-provider", providerConfig.Metadata().Name)
	require.Equal(t, "test/provider", providerConfig.Source)
	require.Equal(t, "1.0.0", providerConfig.Version)
	require.Equal(t, plugin, providerConfig.Plugin)
}

func TestRegisterProviderDuplicateName(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))

	plugin := &RegistryTestPlugin{}
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	registry.RegisterPluginSource("test/provider", plugin)

	provider1 := &resources.Provider{
		Source:  "test/provider",
		Version: "1.0.0",
	}
	provider1.Metadata().Name = "test-provider"
	provider1.Metadata().Type = resources.TypeProvider

	provider2 := &resources.Provider{
		Source:  "test/other",
		Version: "2.0.0",
	}
	provider2.Metadata().Name = "test-provider"
	provider2.Metadata().Type = resources.TypeProvider

	ctx := &hcl.EvalContext{}

	// Register first provider
	err = registry.RegisterProvider(provider1, ctx)
	require.NoError(t, err)

	// Try to register duplicate name
	err = registry.RegisterProvider(provider2, ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already defined")
}

func TestGetProviderNotFound(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))

	_, err := registry.GetProvider("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not defined")
}

func TestGetDefaultProvider(t *testing.T) {
	t.Run("success case", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		plugin := &RegistryTestPlugin{}
		err := registry.RegisterPlugin(plugin)
		require.NoError(t, err)
		registry.RegisterPluginSource("test/provider", plugin)

		// Register a provider with name "container"
		provider := &resources.Provider{
			Source:  "test/provider",
			Version: "1.0.0",
		}
		provider.Metadata().Name = "container"
		provider.Metadata().Type = resources.TypeProvider

		ctx := &hcl.EvalContext{}
		err = registry.RegisterProvider(provider, ctx)
		require.NoError(t, err)

		// Get default provider for "container" resource type
		providerConfig, err := registry.GetDefaultProvider("container")
		require.NoError(t, err)
		require.Equal(t, "container", providerConfig.Metadata().Name)
		require.Equal(t, "test/provider", providerConfig.Source)
		require.Equal(t, "1.0.0", providerConfig.Version)
	})

	t.Run("provider not found", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		// Try to get default provider for unregistered resource type
		_, err := registry.GetDefaultProvider("nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not defined")
	})

	t.Run("empty resource type", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		// Try to get default provider for empty resource type
		_, err := registry.GetDefaultProvider("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not defined")
	})
}

func TestResolveProviderForResource(t *testing.T) {
	// Test resource with explicit provider field  
	type TestResourceWithProvider struct {
		Provider string
		Data     string
	}

	// Test resource without explicit provider field
	type TestResourceWithoutProvider struct {
		Data string
	}

	t.Run("resource with explicit provider field", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		plugin := &RegistryTestPlugin{}
		err := registry.RegisterPlugin(plugin)
		require.NoError(t, err)
		registry.RegisterPluginSource("test/provider", plugin)

		// Register a provider with name "custom-provider"
		provider := &resources.Provider{
			Source:  "test/provider", 
			Version: "1.0.0",
		}
		provider.Metadata().Name = "custom-provider"
		provider.Metadata().Type = resources.TypeProvider

		ctx := &hcl.EvalContext{}
		err = registry.RegisterProvider(provider, ctx)
		require.NoError(t, err)

		// Create resource with explicit provider
		resource := &TestResourceWithProvider{
			Provider: "custom-provider",
			Data:     "test",
		}

		// Resolve provider should return the explicit provider
		providerConfig, err := registry.ResolveProviderForResource(resource)
		require.NoError(t, err)
		require.Equal(t, "custom-provider", providerConfig.Metadata().Name)
	})

	t.Run("resource without provider field falls back to default", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		plugin := &RegistryTestPlugin{}
		err := registry.RegisterPlugin(plugin)
		require.NoError(t, err)
		registry.RegisterPluginSource("test/provider", plugin)

		// Register a provider with name matching resource type
		provider := &resources.Provider{
			Source:  "test/provider",
			Version: "1.0.0",
		}
		provider.Metadata().Name = "TestResourceWithoutProvider" // Should match struct name
		provider.Metadata().Type = resources.TypeProvider

		ctx := &hcl.EvalContext{}
		err = registry.RegisterProvider(provider, ctx)
		require.NoError(t, err)

		// Create resource without provider field
		resource := &TestResourceWithoutProvider{
			Data: "test",
		}

		// Resolve provider should fall back to default based on type
		providerConfig, err := registry.ResolveProviderForResource(resource)
		require.NoError(t, err)
		require.Equal(t, "TestResourceWithoutProvider", providerConfig.Metadata().Name)
	})

	t.Run("explicit provider not found", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		// Create resource with non-existent provider
		resource := &TestResourceWithProvider{
			Provider: "nonexistent-provider",
			Data:     "test",
		}

		// Should return error for non-existent provider
		_, err := registry.ResolveProviderForResource(resource)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not defined")
	})

	t.Run("default provider not found", func(t *testing.T) {
		registry := NewPluginRegistry(logger.NewTestLogger(t))

		// Create resource without provider field and no matching default
		resource := &TestResourceWithoutProvider{
			Data: "test",
		}

		// Should return error for missing default provider
		_, err := registry.ResolveProviderForResource(resource)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not defined")
	})
}

func TestListProviders(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))

	// Initially should be empty
	providers := registry.ListProviders()
	require.Empty(t, providers)

	// Register a plugin source
	plugin := &RegistryTestPlugin{}
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	registry.RegisterPluginSource("test/provider", plugin)
	
	// After registering plugin source, providers list should still be empty
	providers = registry.ListProviders()
	require.Empty(t, providers)

	// Register actual providers
	ctx := &hcl.EvalContext{}

	// Register first provider
	provider1 := &resources.Provider{
		Source:  "test/provider",
		Version: "1.0.0",
	}
	provider1.Metadata().Name = "provider1"
	provider1.Metadata().Type = resources.TypeProvider
	err = registry.RegisterProvider(provider1, ctx)
	require.NoError(t, err)

	// Register second provider
	provider2 := &resources.Provider{
		Source:  "test/provider",
		Version: "2.0.0",
	}
	provider2.Metadata().Name = "provider2"
	provider2.Metadata().Type = resources.TypeProvider
	err = registry.RegisterProvider(provider2, ctx)
	require.NoError(t, err)

	// List should contain exactly our 2 providers (no internal mappings)
	providers = registry.ListProviders()
	require.Len(t, providers, 2)
	require.Contains(t, providers, "provider1")
	require.Contains(t, providers, "provider2")
}

func TestRegisterPluginSource(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))
	plugin := &RegistryTestPlugin{}
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)

	registry.RegisterPluginSource("test/source", plugin)

	// Verify we can find the plugin by source
	foundPlugin, err := registry.findPluginBySource("test/source")
	require.NoError(t, err)
	require.Equal(t, plugin, foundPlugin)
}

func TestFindPluginBySourceNotFound(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))

	_, err := registry.findPluginBySource("nonexistent/source")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestProviderValidationValidBlock(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))
	plugin := &RegistryTestPlugin{}
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	ctx := &hcl.EvalContext{}

	provider := &resources.Provider{
		Source:  "test/provider",
		Version: "1.0.0",
	}
	provider.Metadata().Name = "test"
	provider.Metadata().Type = resources.TypeProvider

	registry.RegisterPluginSource(provider.Source, plugin)
	err = registry.RegisterProvider(provider, ctx)
	require.NoError(t, err)
}

func TestProviderValidationEmptyName(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))
	ctx := &hcl.EvalContext{}

	provider := &resources.Provider{
		Source:  "test/provider",
		Version: "1.0.0",
	}
	provider.Metadata().Name = ""
	provider.Metadata().Type = resources.TypeProvider

	err := registry.RegisterProvider(provider, ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name cannot be empty")
}

func TestProviderValidationEmptySource(t *testing.T) {
	registry := NewPluginRegistry(logger.NewTestLogger(t))
	ctx := &hcl.EvalContext{}

	provider := &resources.Provider{
		Source:  "",
		Version: "1.0.0",
	}
	provider.Metadata().Name = "test"
	provider.Metadata().Type = resources.TypeProvider

	err := registry.RegisterProvider(provider, ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source cannot be empty")
}

// Test fixtures for error handling tests
type ErrorTestConfig struct {
	RequiredField string `hcl:"required_field"`
	OptionalField string `hcl:"optional_field,optional"`
}

type ErrorTestResource struct {
	types.ResourceBase `hcl:",remain"`
	Data               string `hcl:"data"`
}

type ErrorTestProvider struct{}

func (p *ErrorTestProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger plugins.Logger, config ErrorTestConfig) error {
	return nil
}

func (p *ErrorTestProvider) Create(ctx context.Context, resource *ErrorTestResource) (*ErrorTestResource, error) {
	return resource, nil
}

func (p *ErrorTestProvider) Destroy(ctx context.Context, resource *ErrorTestResource, force bool) error {
	return nil
}

func (p *ErrorTestProvider) Refresh(ctx context.Context, resource *ErrorTestResource) error {
	return nil
}

func (p *ErrorTestProvider) Changed(ctx context.Context, current *ErrorTestResource, desired *ErrorTestResource) (bool, error) {
	return false, nil
}

func (p *ErrorTestProvider) Update(ctx context.Context, resource *ErrorTestResource) error {
	return nil
}

func (p *ErrorTestProvider) Functions() plugins.ProviderFunctions {
	return nil
}

type ErrorTestPlugin struct {
	plugins.PluginBase
}


func (p *ErrorTestPlugin) Init(logger plugins.Logger, state plugins.State) error {
	resource := &ErrorTestResource{}
	provider := &ErrorTestProvider{}
	config := ErrorTestConfig{RequiredField: "default", OptionalField: "optional"}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"error_test",
		resource,
		provider,
		config,
	)
}

func TestProviderErrorMissingRequiredField(t *testing.T) {
	// Create temporary test file with missing required field
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "error_test" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    # missing required_field
    optional_field = "present"
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &ErrorTestPlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/error", plugin)

	// Parse should fail due to missing required field
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "required_field") // Should mention the missing field
}

func TestProviderErrorInvalidSource(t *testing.T) {
	// Create temporary test file with invalid source
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "error_test" {
  source = "invalid/source"  # This source is not registered
  version = "1.0.0"
  
  config {
    required_field = "test"
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin with different source
	parser := NewParser(nil)
	plugin := &ErrorTestPlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/error", plugin) // Different source

	// Parse should fail due to source not found
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestProviderErrorMalformedHCL(t *testing.T) {
	// Create temporary test file with malformed HCL
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "error_test" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    required_field = "unclosed string
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &ErrorTestPlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/error", plugin)

	// Parse should fail due to malformed HCL
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	// Should be an HCL parsing error
}

func TestProviderErrorDuplicateProviderNames(t *testing.T) {
	// Create temporary test file with duplicate provider names
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "duplicate" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    required_field = "first"
  }
}

provider "duplicate" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    required_field = "second"
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &ErrorTestPlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/error", plugin)

	// Parse should fail due to duplicate provider names
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already defined")
}

func TestProviderErrorMissingPluginForResource(t *testing.T) {
	// Create temporary test file with resource that has no registered plugin
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
resource "unregistered_type" "test" {
  data = "should fail"
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser without registering the required plugin
	parser := NewParser(nil)

	// Parse should fail due to unregistered resource type
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found") // Should mention type not found
}

func TestProviderErrorInvalidVariableReference(t *testing.T) {
	// Create temporary test file with invalid variable reference
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "error_test" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    required_field = variable.nonexistent_var
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &ErrorTestPlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/error", plugin)

	// Parse should fail due to undefined variable
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "variable") // Should mention variable issue
}

func TestProviderErrorEmptyProvider(t *testing.T) {
	// Create temporary test file with completely empty provider block
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "empty" {
  # No source, version, or config
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser
	parser := NewParser(nil)

	// Parse should fail due to missing required fields
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
}

func TestProviderErrorCircularVariableReference(t *testing.T) {
	// Create temporary test file with circular variable references
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
variable "var1" {
  default = variable.var2
}

variable "var2" {
  default = variable.var1
}

provider "error_test" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    required_field = variable.var1
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &ErrorTestPlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/error", plugin)

	// Parse should fail due to circular reference
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	// The exact error message depends on HCL's cycle detection
}

func TestProviderErrorTrulyUndefinedVariable(t *testing.T) {
	// This test uses a variable that is never defined anywhere
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "simple" {
  source = "test/simple"
  version = "1.0.0"
  
  config {
    value = variable.truly_undefined_variable  # This variable is never defined
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &SimplePlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	parser.GetPluginRegistry().RegisterPluginSource("test/simple", plugin)

	// Parse should fail due to truly undefined variable
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "variable") // Should mention variable issue
	
	t.Logf("Parse failed as expected with undefined variable: %v", err)
}

func TestProviderErrorInvalidSourcePath(t *testing.T) {
	// Test with malformed source path
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.hcl")

	hclContent := `
provider "simple" {
  source = "invalid-source-path"  # Invalid source format
  version = "1.0.0"
  
  config {
    value = "test"
  }
}
`

	err := os.WriteFile(testFile, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Create parser and register plugin
	parser := NewParser(nil)
	plugin := &SimplePlugin{}
	err = parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	// Deliberately NOT registering the source "invalid-source-path"

	// Parse should fail due to source not found
	_, err = parser.ParseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
	
	t.Logf("Parse failed as expected with invalid source: %v", err)
}