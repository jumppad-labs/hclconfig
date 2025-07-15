package plugins_test

import (
	"encoding/json"
	"testing"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for Metadata struct
func TestMetadata_JSONSerialization(t *testing.T) {
	metadata := plugins.Metadata{
		Name:         "test-plugin",
		Version:      "v1.2.3",
		Description:  "Test plugin for unit testing",
		Author:       "Test Author",
		Homepage:     "https://github.com/example/test-plugin",
		License:      "MIT",
		Capabilities: []string{"container", "network"},
		API:          "v1",
		OS:           []string{"linux", "darwin"},
		Arch:         []string{"amd64", "arm64"},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(metadata)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var unmarshaledMetadata plugins.Metadata
	err = json.Unmarshal(jsonData, &unmarshaledMetadata)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, metadata.Name, unmarshaledMetadata.Name)
	assert.Equal(t, metadata.Version, unmarshaledMetadata.Version)
	assert.Equal(t, metadata.Description, unmarshaledMetadata.Description)
	assert.Equal(t, metadata.Author, unmarshaledMetadata.Author)
	assert.Equal(t, metadata.Homepage, unmarshaledMetadata.Homepage)
	assert.Equal(t, metadata.License, unmarshaledMetadata.License)
	assert.Equal(t, metadata.Capabilities, unmarshaledMetadata.Capabilities)
	assert.Equal(t, metadata.API, unmarshaledMetadata.API)
	assert.Equal(t, metadata.OS, unmarshaledMetadata.OS)
	assert.Equal(t, metadata.Arch, unmarshaledMetadata.Arch)
}

func TestMetadata_OptionalFields(t *testing.T) {
	// Test with minimal required fields
	metadata := plugins.Metadata{
		Name:        "minimal-plugin",
		Version:     "v1.0.0",
		Description: "Minimal plugin",
		Author:      "Test Author",
		API:         "v1",
	}

	// Test JSON marshaling with optional fields empty
	jsonData, err := json.Marshal(metadata)
	require.NoError(t, err)

	// Verify JSON doesn't include empty optional fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err)

	// License should be omitted when empty due to omitempty tag
	_, hasLicense := jsonMap["license"]
	assert.False(t, hasLicense, "Expected license field to be omitted when empty")
}

// Tests for PluginBase metadata functionality
func TestPluginBase_Metadata(t *testing.T) {
	pluginBase := &plugins.PluginBase{}

	// Test initial metadata is empty
	metadata := pluginBase.Metadata()
	assert.Empty(t, metadata.Name)
	assert.Empty(t, metadata.Version)

	// Test setting metadata
	expectedMetadata := plugins.Metadata{
		Name:         "test-plugin",
		Version:      "v1.0.0",
		Description:  "Test plugin",
		Author:       "Test Author",
		API:          "v1",
		Capabilities: []string{"test"},
	}

	pluginBase.SetMetadata(expectedMetadata)
	actualMetadata := pluginBase.Metadata()

	assert.Equal(t, expectedMetadata, actualMetadata)
}

// Tests for plugin metadata retrieval
func TestPlugin_MetadataRetrieval(t *testing.T) {
	// Create a test plugin instance
	testPlugin := &hclconfig.TestPlugin{}

	// Test that metadata can be retrieved without initialization
	metadata := testPlugin.Metadata()

	// Verify metadata fields
	assert.Equal(t, "test", metadata.Name)
	assert.Equal(t, "v1.0.0", metadata.Version)
	assert.Equal(t, "Test plugin for HCLConfig testing", metadata.Description)
	assert.Equal(t, "HCLConfig Team", metadata.Author)
	assert.Equal(t, "https://github.com/jumppad-labs/hclconfig", metadata.Homepage)
	assert.Equal(t, "MPL-2.0", metadata.License)
	assert.Equal(t, []string{"container", "sidecar", "network", "template"}, metadata.Capabilities)
	assert.Equal(t, "v1", metadata.API)
	assert.Equal(t, []string{"linux", "darwin", "windows"}, metadata.OS)
	assert.Equal(t, []string{"amd64", "arm64"}, metadata.Arch)
}

func TestEmbeddedPlugin_MetadataRetrieval(t *testing.T) {
	// Create an embedded test plugin instance
	embeddedPlugin := &hclconfig.EmbeddedTestPlugin{}

	// Test that metadata can be retrieved without initialization
	metadata := embeddedPlugin.Metadata()

	// Verify metadata fields
	assert.Equal(t, "embedded-test", metadata.Name)
	assert.Equal(t, "v1.0.0", metadata.Version)
	assert.Equal(t, "Embedded test plugin for HCLConfig testing", metadata.Description)
	assert.Equal(t, "HCLConfig Team", metadata.Author)
	assert.Equal(t, "https://github.com/jumppad-labs/hclconfig", metadata.Homepage)
	assert.Equal(t, "MPL-2.0", metadata.License)
	assert.Equal(t, []string{"container", "sidecar"}, metadata.Capabilities)
	assert.Equal(t, "v1", metadata.API)
	assert.Equal(t, []string{"linux", "darwin", "windows"}, metadata.OS)
	assert.Equal(t, []string{"amd64", "arm64"}, metadata.Arch)
}

func TestPluginMetadata_CanBeRetrievedBeforeInit(t *testing.T) {
	// This test verifies that metadata can be retrieved without calling Init()
	// This is important for registry systems that need to query metadata without
	// fully initializing the plugin
	
	testPlugin := &hclconfig.TestPlugin{}
	
	// Get metadata before initialization
	metadata := testPlugin.Metadata()
	
	// Verify we get valid metadata
	require.NotEmpty(t, metadata.Name)
	require.NotEmpty(t, metadata.Version)
	require.NotEmpty(t, metadata.API)
	
	// Verify this works without errors
	assert.Equal(t, "test", metadata.Name)
	assert.Equal(t, "v1", metadata.API)
}