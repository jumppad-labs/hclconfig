package xcl

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	statemocks "github.com/jumppad-labs/xcl/state/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupQueryConfig builds a Config whose registry has the registered type
// database and the plugin types of the TestPlugin (container, network,
// template, sidecar), then applies the query fixture, which holds three
// database blocks and two network blocks.
func setupQueryConfig(t *testing.T) *Config {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	log := logger.NewTestLogger(t)

	pr := registry.NewPluginRegistry(log)

	err := pr.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	ss := &statemocks.MockStateStore{}
	ss.On("Exists").Return(false)
	ss.On("Load").Return(nil, nil)
	ss.On("Save", mock.Anything).Return(nil)

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(ss),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

// TestFindResourcesByTypeListsRegisteredType asserts that listing a
// registered type returns exactly the resources of that type, as the
// registered Go type.
func TestFindResourcesByTypeListsRegisteredType(t *testing.T) {
	c := setupQueryConfig(t)

	q := NewQuerier[registered.Database](c)

	databases, err := q.FindResourcesByType(registered.TypeDatabase)
	require.NoError(t, err)
	require.Len(t, databases, 3)

	ids := []string{}
	for _, db := range databases {
		ids = append(ids, db.Meta.ID)
	}
	sort.Strings(ids)

	require.Equal(t, []string{
		"resource.database.archive",
		"resource.database.primary",
		"resource.database.replica",
	}, ids)
}

// TestFindResourcesByTypeListsPluginType asserts that listing a plugin type
// returns exactly the resources of that type, filled in from the plugin's
// generated resources.
func TestFindResourcesByTypeListsPluginType(t *testing.T) {
	c := setupQueryConfig(t)

	q := NewQuerier[structs.Network](c)

	networks, err := q.FindResourcesByType(structs.TypeNetwork)
	require.NoError(t, err)
	require.Len(t, networks, 2)

	subnets := map[string]string{}
	for _, n := range networks {
		subnets[n.Meta.ID] = n.Subnet
	}

	require.Equal(t, map[string]string{
		"resource.network.backend":  "10.0.2.0/24",
		"resource.network.frontend": "10.0.1.0/24",
	}, subnets)
}

// TestFindResourcesByTypeReturnsNotFoundForUnusedType asserts that listing a
// type with no resources returns a not found error.
func TestFindResourcesByTypeReturnsNotFoundForUnusedType(t *testing.T) {
	c := setupQueryConfig(t)

	q := NewQuerier[structs.Container](c)

	containers, err := q.FindResourcesByType("container")
	require.Error(t, err)
	require.ErrorAs(t, err, &state.ResourceNotFoundError{})
	require.Nil(t, containers)
}

// TestFindResourceReturnsStoredRegisteredValue asserts that finding a
// registered resource by path returns the value held in state, not a copy.
func TestFindResourceReturnsStoredRegisteredValue(t *testing.T) {
	c := setupQueryConfig(t)

	stored, err := c.FindResource("resource.database.primary")
	require.NoError(t, err)

	q := NewQuerier[registered.Database](c)

	db, err := q.FindResource("resource.database.primary")
	require.NoError(t, err)

	require.Same(t, stored, db)
	require.Equal(t, "eu-west", db.Location)
	require.Equal(t, 5432, db.Port)
}

// TestFindResourcesByTypeReturnsStoredRegisteredValues asserts that listing a
// registered type returns the values held in state, not copies.
func TestFindResourcesByTypeReturnsStoredRegisteredValues(t *testing.T) {
	c := setupQueryConfig(t)

	q := NewQuerier[registered.Database](c)

	databases, err := q.FindResourcesByType(registered.TypeDatabase)
	require.NoError(t, err)

	for _, db := range databases {
		stored, err := c.FindResource(db.Meta.ID)
		require.NoError(t, err)
		require.Same(t, stored, db)
	}
}

// TestFindResourceCopiesPluginResource asserts that finding a plugin resource
// by path returns a filled-in copy of the plugin's generated resource.
func TestFindResourceCopiesPluginResource(t *testing.T) {
	c := setupQueryConfig(t)

	stored, err := c.FindResource("resource.network.frontend")
	require.NoError(t, err)

	q := NewQuerier[structs.Network](c)

	network, err := q.FindResource("resource.network.frontend")
	require.NoError(t, err)

	_, isNetwork := stored.(*structs.Network)
	require.False(t, isNetwork, "plugin resources are held as generated types, not the plugin's Go type")

	require.Equal(t, "resource.network.frontend", network.Meta.ID)
	require.Equal(t, "10.0.1.0/24", network.Subnet)
}

// TestFindResourceReturnsNotFoundForUnknownPath asserts that finding a path
// with no resource returns a not found error.
func TestFindResourceReturnsNotFoundForUnknownPath(t *testing.T) {
	c := setupQueryConfig(t)

	q := NewQuerier[registered.Database](c)

	_, err := q.FindResource("resource.database.missing")
	require.Error(t, err)
	require.ErrorAs(t, err, &state.ResourceNotFoundError{})
}
