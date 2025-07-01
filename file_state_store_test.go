package hclconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func TestFileStateStoreCreatesStateDirectoryIfNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	stateDir := filepath.Join(tmpDir, "test-state")
	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore(stateDir, registry)

	// Save should create the directory
	config := &Config{Resources: []types.Resource{}}
	err = store.Save(config)
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(stateDir)
	require.NoError(t, err)
}

func TestFileStateStoreSavesAndLoadsConfigCorrectly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore(filepath.Join(tmpDir, "save-load-test"), registry)

	// Create test config with a builtin resource (variable)
	config := &Config{
		Resources: []types.Resource{
			&types.ResourceBase{
				Meta: types.Meta{
					ID:   "variable.example",
					Name: "example", 
					Type: "variable",
					Properties: map[string]any{
						"default": "test-value",
					},
				},
			},
		},
	}

	// Save the state
	err = store.Save(config)
	require.NoError(t, err)

	// Load the state
	loadedConfig, err := store.Load()
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	require.Len(t, loadedConfig.Resources, 1)

	// Verify the content matches
	resource := loadedConfig.Resources[0]
	require.Equal(t, "variable.example", resource.Metadata().ID)
	require.Equal(t, "example", resource.Metadata().Name)
	require.Equal(t, "variable", resource.Metadata().Type)
}

func TestFileStateStoreReturnsNilWhenNoStateExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore(filepath.Join(tmpDir, "no-state-test"), registry)

	config, err := store.Load()
	require.NoError(t, err)
	require.Nil(t, config)
}

func TestFileStateStoreExistsReturnsCorrectStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore(filepath.Join(tmpDir, "exists-test"), registry)

	// Should not exist initially
	require.False(t, store.Exists())

	// Save state
	config := &Config{Resources: []types.Resource{}}
	err = store.Save(config)
	require.NoError(t, err)

	// Should exist now
	require.True(t, store.Exists())
}

func TestFileStateStoreClearRemovesStateFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore(filepath.Join(tmpDir, "clear-test"), registry)

	// Save state
	config := &Config{Resources: []types.Resource{}}
	err = store.Save(config)
	require.NoError(t, err)
	require.True(t, store.Exists())

	// Clear state
	err = store.Clear()
	require.NoError(t, err)
	require.False(t, store.Exists())

	// Clear again should not error
	err = store.Clear()
	require.NoError(t, err)
}

func TestFileStateStoreHandlesConcurrentAccessSafely(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore(filepath.Join(tmpDir, "concurrent-test"), registry)

	// Run multiple goroutines saving and loading
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			config := &Config{
				Resources: []types.Resource{
					&types.ResourceBase{
						Meta: types.Meta{
							ID:   "test.concurrent",
							Name: "concurrent",
							Type: "variable",
						},
					},
				},
			}
			
			err := store.Save(config)
			require.NoError(t, err)

			loaded, err := store.Load()
			require.NoError(t, err)
			require.NotNil(t, loaded)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFileStateStoreUsesDefaultDirectoryWhenEmptyStringProvided(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to temp directory to avoid polluting project
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	registry := NewResourceRegistry([]plugins.PluginHost{})
	store := NewFileStateStore("", registry)

	// Save state
	config := &Config{Resources: []types.Resource{}}
	err = store.Save(config)
	require.NoError(t, err)

	// Verify state was saved in default location
	_, err = os.Stat(filepath.Join(".hclconfig", "state", "state.json"))
	require.NoError(t, err)

	// Clean up
	os.RemoveAll(".hclconfig")
}