package hclconfig

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func setupGraphConfig(t *testing.T) *Config {
	absoluteFolderPath, err := filepath.Abs("./internal/test_fixtures/config/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupTestParserWithLogger(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	return c
}

func setupTestParserWithLogger(t *testing.T) *Parser {
	opts := DefaultOptions()
	opts.Logger = logger.NewTestLogger(t)
	p := NewParser(opts)
	
	// Create and register the test plugin
	testPlugin := &TestPlugin{}
	err := p.RegisterPlugin(testPlugin)
	if err != nil {
		t.Fatal("Failed to register test plugin:", err)
	}
	
	return p
}

func TestDoYaLikeDAGAddsDependencies(t *testing.T) {
	c := setupGraphConfig(t)

	g, err := doYaLikeDAGs(c)
	require.NoError(t, err)

	network, err := c.FindResource("resource.network.onprem")
	require.NoError(t, err)

	template, err := c.FindResource("resource.template.consul_config")
	require.NoError(t, err)

	// check the dependency tree of the base container
	base, err := c.FindResource("resource.container.base")
	require.NoError(t, err)

	s, err := g.Descendents(base)
	require.NoError(t, err)

	// check the network is returned
	list := s.List()
	require.Contains(t, list, network)

	// check the dependency tree of the consul container
	consul, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	s, err = g.Descendents(consul)
	require.NoError(t, err)

	// check the network is returned
	list = s.List()
	require.Contains(t, list, network)
	require.Contains(t, list, base)
	require.Contains(t, list, template)
}

func TestProviderResourceDependency(t *testing.T) {
	// Use test fixture for provider-resource dependency
	testFile := "./internal/test_fixtures/config/providers/basic_provider_with_resources.hcl"

	// Create parser and register plugin
	parser, _ := setupParser(t)
	plugin := &SimplePlugin{}
	err := parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	// Register plugin source mapping
	parser.GetPluginRegistry().RegisterPluginSource("test/simple", plugin)

	// Parse the file (which includes processing and should handle dependencies)
	config, err := parser.ParseFile(testFile)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify provider was initialized properly
	providerConfig, err := parser.GetPluginRegistry().GetProvider("simple")
	require.NoError(t, err)
	require.True(t, providerConfig.Initialized, "Provider should be initialized")
	
	// Verify config was resolved from variable
	configValue, ok := providerConfig.Config.(*SimpleConfig)
	require.True(t, ok, "Config should be of type SimpleConfig")
	require.Equal(t, "from_variable", configValue.Value, "Value should be resolved from variable")
	require.Equal(t, 42, configValue.Count, "Count should be set")

	// Verify resource exists and was processed
	var simpleResources []types.Resource
	for _, r := range config.Resources {
		if r.Metadata().Type == "simple" {
			simpleResources = append(simpleResources, r)
		}
	}
	require.Len(t, simpleResources, 1)
	resource := simpleResources[0]
	require.Equal(t, "simple", resource.Metadata().Type)
	require.Equal(t, "test", resource.Metadata().Name)
}

func TestProviderResourceDependencyInDAG(t *testing.T) {
	// Use test fixture for DAG provider-resource dependencies
	testFile := "./internal/test_fixtures/config/providers/provider_with_resources.hcl"

	// Create parser and register plugin
	parser, _ := setupParser(t)
	plugin := &SimplePlugin{}
	err := parser.RegisterPlugin(plugin)
	require.NoError(t, err)

	// Register plugin source mapping  
	parser.GetPluginRegistry().RegisterPluginSource("test/simple", plugin)

	// Parse the file
	config, err := parser.ParseFile(testFile)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify that the provider was processed before the resources
	// This test demonstrates that the DAG correctly handles provider dependencies
	
	// Check provider was initialized
	providerConfig, err := parser.GetPluginRegistry().GetProvider("simple")
	require.NoError(t, err)
	require.True(t, providerConfig.Initialized, "Provider should be initialized")
	
	// Check config was resolved
	configValue, ok := providerConfig.Config.(*SimpleConfig)
	require.True(t, ok)
	require.Equal(t, "https://api.example.com", configValue.Value)
	require.Equal(t, 100, configValue.Count)

	// Check both resources exist and were processed
	var simpleResources []types.Resource
	for _, r := range config.Resources {
		if r.Metadata().Type == "simple" {
			simpleResources = append(simpleResources, r)
		}
	}
	require.Len(t, simpleResources, 2)
	
	// Both resources should have been processed (status should not be empty)
	for _, r := range simpleResources {
		require.NotEmpty(t, r.Metadata().Status, "Resource should have been processed")
	}
}
