package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// configDir is the shared example configuration
const configDir = "../config"

// declaredResourceIDs is every resource the shared example configuration
// declares, sorted
var declaredResourceIDs = []string{
	"module.analytics",
	"module.analytics.output.location",
	"module.analytics.resource.postgres.analytics",
	"module.analytics.variable.db_username",
	"output.web_database",
	"resource.app.web",
	"resource.postgres.main",
	"resource.postgres.replica",
	"variable.db_password",
	"variable.db_username",
}

func resourceIDs(t *testing.T, found []any) []string {
	t.Helper()

	ids := []string{}
	for _, r := range found {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)
	return ids
}

func findResource(t *testing.T, found []any, id string) any {
	t.Helper()

	for _, r := range found {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		if meta.ID == id {
			return r
		}
	}

	require.Failf(t, "resource not found", "no resource with id %s", id)
	return nil
}

func TestConfigOnlyExampleFindsDeclaredResources(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, configDir)
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

func TestConfigOnlyExampleReturnsRegisteredGoTypes(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, configDir)
	require.NoError(t, err)

	db, ok := findResource(t, found, "resource.postgres.main").(*resources.PostgreSQL)
	require.True(t, ok)
	require.Equal(t, "localhost", db.Location)
	require.Equal(t, 5432, db.Port)
	require.Equal(t, "admin", db.Username)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 10, db.Timeouts.Connection)
	require.Equal(t, 60, db.Timeouts.KeepAlive)

	app, ok := findResource(t, found, "resource.app.web").(*resources.App)
	require.True(t, ok)
	require.Equal(t, "localhost", app.DatabaseLocation)
	require.Equal(t, "admin", app.DatabaseUser)
	require.Equal(t, "analytics.localhost", app.AnalyticsLocation)
}

func TestConfigOnlyExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, configDir)
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

func TestConfigOnlyExamplePrintsQueryResult(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, configDir)
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Databases\n")
	require.Contains(t, out.String(), `resource.postgres.main location=localhost port=5432 connection_string=""`)
	require.Contains(t, out.String(), `resource.postgres.replica location=replica.localhost port=5433 connection_string=""`)
	require.Contains(t, out.String(), `module.analytics.resource.postgres.analytics location=analytics.localhost port=5432 connection_string=""`)
	require.Contains(t, out.String(), "## App\n")
	require.Contains(t, out.String(), "resource.app.web database_location=localhost database_user=admin analytics_location=analytics.localhost")
}

func TestConfigOnlyExampleLeavesConnectionStringEmpty(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, configDir)
	require.NoError(t, err)

	db, ok := findResource(t, found, "resource.postgres.main").(*resources.PostgreSQL)
	require.True(t, ok)
	require.Empty(t, db.ConnectionString)

	app, ok := findResource(t, found, "resource.app.web").(*resources.App)
	require.True(t, ok)
	require.Empty(t, app.ConnectionString)
}

func TestConfigOnlyExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, "./does-not-exist")
	require.Error(t, err)
}

// TestConfigOnlyExampleImportsNoPluginCode asserts the configuration only
// example uses no plugin or provider code, the plugin registry is the only
// package it may use from plugins
func TestConfigOnlyExampleImportsNoPluginCode(t *testing.T) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				require.NoError(t, err)

				if path == "github.com/jumppad-labs/xcl/plugins/registry" {
					continue
				}

				require.False(t,
					strings.HasPrefix(path, "github.com/jumppad-labs/xcl/plugins"),
					"%s imports plugin code %s", name, path,
				)
			}
		}
	}
}
