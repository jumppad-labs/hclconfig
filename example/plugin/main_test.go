package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// configDir is the shared example configuration
const configDir = "../config"

// declaredResourceIDs is every resource the shared example configuration
// declares, sorted, the same set the configonly example finds
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

func TestPluginExampleFindsDeclaredResources(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, configDir)
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

func TestPluginExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, configDir)
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

func TestPluginExampleFillsConnectionString(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, configDir)
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.postgres.main location=localhost port=5432 connection_string="postgres://admin@localhost:5432/main"`)
	require.Contains(t, out.String(), `resource.postgres.replica location=replica.localhost port=5433 connection_string="postgres://admin@replica.localhost:5433/main"`)
	require.Contains(t, out.String(), `module.analytics.resource.postgres.analytics location=analytics.localhost port=5432 connection_string="postgres://analytics@analytics.localhost:5432/analytics"`)
}

func TestPluginExamplePassesConnectionStringToReferencingBlock(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, configDir)
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.app.web database_location=localhost database_user=admin analytics_location=analytics.localhost connection_string="postgres://admin@localhost:5432/main"`)
}

// TestPluginExampleHoldsGeneratedTypes asserts plugin resources are held as
// types generated from the plugin's schema, so the query results printed by
// the example were copied into the shared Go types
func TestPluginExampleHoldsGeneratedTypes(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, configDir)
	require.NoError(t, err)

	for _, r := range found {
		_, isPostgres := r.(*resources.PostgreSQL)
		require.False(t, isPostgres, "plugin resources are held as generated types")

		_, isApp := r.(*resources.App)
		require.False(t, isApp, "plugin resources are held as generated types")
	}
}

func TestPluginExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, "./does-not-exist")
	require.Error(t, err)
}

// TestPluginExampleDefinesNoTypesOrConfig asserts the plugin example uses the
// shared configuration and Go types, it declares no resource type and holds
// no configuration of its own
func TestPluginExampleDefinesNoTypesOrConfig(t *testing.T) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}

				for _, field := range st.Fields.List {
					sel, ok := field.Type.(*ast.SelectorExpr)
					if !ok {
						continue
					}

					require.NotEqual(t, "ResourceBase", sel.Sel.Name,
						"%s declares resource type %s, use the shared example/resources types", name, ts.Name.Name)
				}

				return true
			})
		}
	}

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	for _, e := range entries {
		require.NotEqual(t, ".xcl", filepath.Ext(e.Name()),
			"%s is configuration, use the shared example/config", e.Name())
	}
}
