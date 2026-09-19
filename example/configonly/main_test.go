package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
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

	found, err := run(out, logger.NewTestLogger(t), configDir)
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

func TestConfigOnlyExampleReturnsRegisteredGoTypes(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, logger.NewTestLogger(t), configDir)
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

	_, err := run(out, logger.NewTestLogger(t), configDir)
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

func TestConfigOnlyExamplePrintsQueryResult(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir)
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

	found, err := run(out, logger.NewTestLogger(t), configDir)
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

	_, err := run(out, logger.NewTestLogger(t), "./does-not-exist")
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

// loggedMessage is one call to a recordingLogger
type loggedMessage struct {
	level string
	msg   string
	args  []any
}

// recordingLogger records every message logged to it, events are fired from
// concurrent walk goroutines so recording is guarded by a mutex
type recordingLogger struct {
	mu       sync.Mutex
	messages []loggedMessage
}

func (l *recordingLogger) record(level, msg string, args []any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.messages = append(l.messages, loggedMessage{level: level, msg: msg, args: args})
}

func (l *recordingLogger) Info(msg string, args ...any)  { l.record("info", msg, args) }
func (l *recordingLogger) Debug(msg string, args ...any) { l.record("debug", msg, args) }
func (l *recordingLogger) Warn(msg string, args ...any)  { l.record("warn", msg, args) }
func (l *recordingLogger) Error(msg string, args ...any) { l.record("error", msg, args) }

// events returns the args of every event logged at level for operation
func (l *recordingLogger) events(level, operation string) [][]any {
	l.mu.Lock()
	defer l.mu.Unlock()

	found := [][]any{}
	for _, m := range l.messages {
		if m.level == level && m.msg == "" && m.args[1] == operation {
			found = append(found, m.args)
		}
	}

	return found
}

// TestConfigOnlyExampleLogsParseEventWithFileAtDebug asserts each resource's
// parse event is logged at debug with the file it was parsed from
func TestConfigOnlyExampleLogsParseEventWithFileAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir)
	require.NoError(t, err)

	files := map[string]string{}
	for _, args := range log.events("debug", "parse") {
		require.Equal(t, []any{"event", "parse", "resource", args[3], "file", args[5], "phase", "success"}, args)
		files[args[3].(string)] = filepath.Base(args[5].(string))
	}

	require.Equal(t, map[string]string{
		"variable.db_username":                         "main.xcl",
		"variable.db_password":                         "main.xcl",
		"resource.postgres.main":                       "main.xcl",
		"resource.postgres.replica":                    "main.xcl",
		"module.analytics":                             "main.xcl",
		"resource.app.web":                             "main.xcl",
		"output.web_database":                          "main.xcl",
		"module.analytics.variable.db_username":        "db.xcl",
		"module.analytics.resource.postgres.analytics": "db.xcl",
		"module.analytics.output.location":             "db.xcl",
	}, files)
}

// TestConfigOnlyExampleLogsCreateSuccessWithoutStartAtDebug asserts every
// resource's create is logged at debug as a success only, registered types
// have no provider so there is nothing to start
func TestConfigOnlyExampleLogsCreateSuccessWithoutStartAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir)
	require.NoError(t, err)

	ids := []string{}
	for _, args := range log.events("debug", "create") {
		require.Equal(t, "success", args[5])
		ids = append(ids, args[3].(string))
	}
	sort.Strings(ids)

	require.Equal(t, declaredResourceIDs, ids)
}

// TestConfigOnlyExampleLogsNothingAtInfo asserts a successful run logs
// nothing above debug, only an error would stand out
func TestConfigOnlyExampleLogsNothingAtInfo(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir)
	require.NoError(t, err)

	for _, m := range log.messages {
		require.Equal(t, "debug", m.level, "unexpected %s message: %s %v", m.level, m.msg, m.args)
	}
}

// TestConfigOnlyExampleLogsParseErrorAtError asserts a block that fails to
// parse is logged at error, with its file
func TestConfigOnlyExampleLogsParseErrorAtError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.xcl")
	err := os.WriteFile(file, []byte(`resource "nosuchtype" "broken" {}`), 0644)
	require.NoError(t, err)

	log := &recordingLogger{}

	_, err = run(&bytes.Buffer{}, log, dir)
	require.Error(t, err)

	failed := log.events("error", "parse")
	require.Len(t, failed, 1)
	require.Equal(t, "resource.nosuchtype.broken", failed[0][3])
	require.Equal(t, file, failed[0][5])
}
