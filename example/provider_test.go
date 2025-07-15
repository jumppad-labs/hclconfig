package main

import (
	"testing"

	"github.com/jumppad-labs/hclconfig"
	"github.com/stretchr/testify/require"
)

func TestProviderBlockParsing(t *testing.T) {
	// Create a new parser
	parser := hclconfig.NewParser(nil)
	
	// Create and register the containerd plugin
	containerdPlugin := &ContainerdPlugin{}
	err := parser.RegisterPlugin(containerdPlugin)
	require.NoError(t, err, "Should be able to register containerd plugin")
	
	// Register the plugin source mapping
	parser.GetPluginRegistry().RegisterPluginSource("jumppad/containerd", containerdPlugin)
	
	// Parse the example configuration
	config, err := parser.ParseFile("./provider_example.hcl")
	require.NoError(t, err, "Should be able to parse provider configuration")
	require.NotNil(t, config, "Config should not be nil")
	
	// Verify providers were parsed
	providers := parser.GetPluginRegistry().ListProviders()
	require.Contains(t, providers, "container", "Should have registered the container provider")
	
	// Verify provider configuration
	provider, err := parser.GetPluginRegistry().GetProvider("container")
	require.NoError(t, err, "Should be able to get container provider")
	require.Equal(t, "jumppad/containerd", provider.Source, "Provider source should match")
	require.Equal(t, "~> 1.0", provider.Version, "Provider version should match")
	
	// Verify config was parsed correctly
	config_val, ok := provider.Config.(*ContainerdConfig)
	require.True(t, ok, "Config should be of type ContainerdConfig")
	
	t.Logf("Config values: Socket=%s, Namespace=%s, Snapshotter=%s, Runtime=%s", 
		config_val.Socket, config_val.Namespace, config_val.Snapshotter, config_val.Runtime)
	
	require.Equal(t, "/run/containerd/containerd.sock", config_val.Socket, "Socket should be set from variable")
	require.Equal(t, "default", config_val.Namespace, "Namespace should be set")
	require.Equal(t, "overlayfs", config_val.Snapshotter, "Snapshotter should be set")
	require.Equal(t, "runc", config_val.Runtime, "Runtime should be set")
}