package state

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/stretchr/testify/require"
)

func testCreateState(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	p := path.Join(t.TempDir(), "state.json")
	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	ss, err := NewFileStateStore(p, reg)

	require.NoError(t, err)
	require.NotNil(t, ss)
	require.FileExists(t, p)

	return ss, p, reg
}

func testSaveState(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	ss, p, reg := testCreateState(t)
	s := NewState()

	// Create a variable resource using the registry
	varResource, err := reg.CreateResource(resources.TypeVariable, "example")
	require.NoError(t, err)

	err = s.AppendResource(varResource)
	require.NoError(t, err)

	err = ss.Save(s)
	require.NoError(t, err)
	require.FileExists(t, p)

	return ss, p, reg
}

func testNewStateAtExistingPath(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	_, p, reg := testSaveState(t)

	// create the file first
	err := os.WriteFile(p, []byte("{}"), 0644)
	require.NoError(t, err)

	ss, err := NewFileStateStore(p, reg)

	require.NoError(t, err)
	require.NotNil(t, ss)

	return ss, p, reg
}

func TestCreatesStateAtEmptyPath(t *testing.T) {
	testCreateState(t)
}

func TestSaveSavesStateToFile(t *testing.T) {
	_, p, _ := testSaveState(t)

	// load the file and check contents
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	res := []*resources.Variable{}
	err = json.Unmarshal(data, &res)
	require.NoError(t, err)
	require.Equal(t, "variable.example", res[0].Meta.ID)
}

func TestNewStateAtExistingPath(t *testing.T) {
	testNewStateAtExistingPath(t)
}

func TestLoadStateContainsResources(t *testing.T) {
	ss, _, _ := testSaveState(t)

	s, err := ss.Load()
	require.NoError(t, err)
	require.NotNil(t, s)

	_, err = s.FindResource("variable.example")
	require.NoError(t, err)
}
