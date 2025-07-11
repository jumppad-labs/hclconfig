package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/state"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func TestFileStateStoreCreatesStateDirectoryIfNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	stateDir := filepath.Join(tmpDir, "test-state")
	store := state.NewFileStateStore(stateDir, func() any { return hclconfig.NewConfig() })

	// Save should create the directory
	config := &hclconfig.Config{Resources: []types.Resource{}}
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

	store := state.NewFileStateStore(filepath.Join(tmpDir, "save-load-test"), func() any { return hclconfig.NewConfig() })

	// Create test config with a builtin resource (variable)
	config := &hclconfig.Config{
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
	loadedState, err := store.Load()
	require.NoError(t, err)
	require.NotNil(t, loadedState)
	
	// Type assert to *hclconfig.Config
	loadedConfig, ok := loadedState.(*hclconfig.Config)
	require.True(t, ok)
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

	store := state.NewFileStateStore(filepath.Join(tmpDir, "no-state-test"), func() any { return hclconfig.NewConfig() })

	loadedState, err := store.Load()
	require.NoError(t, err)
	require.Nil(t, loadedState)
}

func TestFileStateStoreExistsReturnsCorrectStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := state.NewFileStateStore(filepath.Join(tmpDir, "exists-test"), func() any { return hclconfig.NewConfig() })

	// Should not exist initially
	require.False(t, store.Exists())

	// Save state
	config := &hclconfig.Config{Resources: []types.Resource{}}
	err = store.Save(config)
	require.NoError(t, err)

	// Should exist now
	require.True(t, store.Exists())
}

func TestFileStateStoreClearRemovesStateFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hclconfig-state-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := state.NewFileStateStore(filepath.Join(tmpDir, "clear-test"), func() any { return hclconfig.NewConfig() })

	// Save state
	config := &hclconfig.Config{Resources: []types.Resource{}}
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

	store := state.NewFileStateStore(filepath.Join(tmpDir, "concurrent-test"), func() any { return hclconfig.NewConfig() })

	// Run multiple goroutines saving and loading
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			config := &hclconfig.Config{
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

			loadedState, err := store.Load()
			require.NoError(t, err)
			require.NotNil(t, loadedState)
			
			// Type assert to *hclconfig.Config
			_, ok := loadedState.(*hclconfig.Config)
			require.True(t, ok)

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

	store := state.NewFileStateStore("", func() any { return hclconfig.NewConfig() })

	// Save state
	config := &hclconfig.Config{Resources: []types.Resource{}}
	err = store.Save(config)
	require.NoError(t, err)

	// Verify state was saved in default location
	_, err = os.Stat(filepath.Join(".hclconfig", "state", "state.json"))
	require.NoError(t, err)

	// Clean up
	os.RemoveAll(".hclconfig")
}