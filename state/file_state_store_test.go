package state

import (
	"encoding/json"
	"errors"
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

// stateWithUnknownTypes is a saved state holding a known variable alongside
// resources whose types nothing registers: two postgres databases and a redis
// cache.
const stateWithUnknownTypes = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "resource.redis.cache", "type": "redis", "name": "cache"}
  },
  {
    "meta": {"id": "resource.postgres.main", "type": "postgres", "name": "main"}
  },
  {
    "meta": {"id": "resource.postgres.replica", "type": "postgres", "name": "replica"}
  }
]`

// stateWithUnknownType is a saved state holding a known variable alongside a
// postgres resource whose type nothing registers.
const stateWithUnknownType = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "resource.postgres.main", "type": "postgres", "name": "main"}
  }
]`

// Loading a state holding a type nobody registered fails naming that type,
// rather than returning a state that silently drops the resource.
func TestLoadFailsWhenStateHoldsUnknownType(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithUnknownType), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"postgres"}, unknown.Types)
	require.Contains(t, err.Error(), "postgres")
}

// Every unknown type is named once, in sorted order, however many resources
// of that type the state holds.
func TestLoadNamesEachUnknownTypeOnceInSortedOrder(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithUnknownTypes), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"postgres", "redis"}, unknown.Types)
}
