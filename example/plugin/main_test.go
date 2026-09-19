package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// configDir is the shared example configuration
const configDir = "../config"

// externalPlugin is the external plugin binary, built once for the package's
// tests by TestMain
var externalPlugin string

// TestMain builds the external plugin into a temporary directory, so the
// tests never depend on a binary built by hand
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "xcl-example-plugin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create build directory: %s\n", err)
		os.Exit(1)
	}

	externalPlugin = filepath.Join(dir, "external")

	build := exec.Command("go", "build", "-o", externalPlugin, "./external")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to build the external plugin: %s\n%s", err, output)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

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

// loggedMessage is one call to a recordingLogger
type loggedMessage struct {
	level string
	msg   string
	args  []any
}

// recordingLogger records every message logged to it, providers and the event
// handler log from concurrent walk goroutines so recording is guarded by a
// mutex
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

// withMessage returns every message with the given level and text
func (l *recordingLogger) withMessage(level, msg string) []loggedMessage {
	l.mu.Lock()
	defer l.mu.Unlock()

	found := []loggedMessage{}
	for _, m := range l.messages {
		if m.level == level && m.msg == msg {
			found = append(found, m)
		}
	}

	return found
}

// argsOf returns the args of each message, so tests can compare them without
// depending on the order resources were processed in
func argsOf(messages []loggedMessage) [][]any {
	args := [][]any{}
	for _, m := range messages {
		args = append(args, m.args)
	}

	return args
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

	found, err := run(out, logger.NewTestLogger(t), configDir, externalPlugin)
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

func TestPluginExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, externalPlugin)
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

func TestPluginExampleFillsConnectionString(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, externalPlugin)
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.postgres.main location=localhost port=5432 connection_string="postgres://admin@localhost:5432/main"`)
	require.Contains(t, out.String(), `resource.postgres.replica location=replica.localhost port=5433 connection_string="postgres://admin@replica.localhost:5433/main"`)
	require.Contains(t, out.String(), `module.analytics.resource.postgres.analytics location=analytics.localhost port=5432 connection_string="postgres://analytics@analytics.localhost:5432/analytics"`)
}

func TestPluginExamplePassesConnectionStringToReferencingBlock(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, externalPlugin)
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.app.web database_location=localhost database_user=admin analytics_location=analytics.localhost connection_string="postgres://admin@localhost:5432/main"`)
}

// TestPluginExampleHoldsGeneratedTypes asserts plugin resources are held as
// types generated from the plugin's schema, so the query results printed by
// the example were copied into the shared Go types
func TestPluginExampleHoldsGeneratedTypes(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, logger.NewTestLogger(t), configDir, externalPlugin)
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

	_, err := run(out, logger.NewTestLogger(t), "./does-not-exist", externalPlugin)
	require.Error(t, err)
}

// TestPluginExampleDefinesNoTypesOrConfig asserts the plugin example and its
// external plugin use the shared configuration and Go types, they declare no
// resource type and hold no configuration of their own
func TestPluginExampleDefinesNoTypesOrConfig(t *testing.T) {
	for _, dir := range []string{".", "./internal", "./external"} {
		fset := token.NewFileSet()

		pkgs, err := parser.ParseDir(fset, dir, nil, 0)
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

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)

		for _, e := range entries {
			require.NotEqual(t, ".xcl", filepath.Ext(e.Name()),
				"%s is configuration, use the shared example/config", filepath.Join(dir, e.Name()))
		}
	}
}

func TestPluginExampleLogsInProcessPluginInitAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	require.Len(t, log.withMessage("debug", "plugin=ExamplePlugin init"), 1)
}

func TestPluginExampleLogsInProcessProviderInitAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	require.Len(t, log.withMessage("debug", "plugin=ExamplePlugin provider=postgres init"), 1)
}

func TestPluginExampleLogsInProcessProviderCreateAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	require.ElementsMatch(t, [][]any{
		{"id", "resource.postgres.main", "connection_string", "postgres://admin@localhost:5432/main"},
		{"id", "resource.postgres.replica", "connection_string", "postgres://admin@replica.localhost:5433/main"},
		{"id", "module.analytics.resource.postgres.analytics", "connection_string", "postgres://analytics@analytics.localhost:5432/analytics"},
	}, argsOf(log.withMessage("debug", "plugin=ExamplePlugin provider=postgres create")))
}

// TestPluginExampleLogsExternalProviderCreateAtDebug asserts a log from the external plugin process reaches the host logger,
// tagged with the plugin binary's name and the provider's block type
func TestPluginExampleLogsExternalProviderCreateAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	require.Equal(t, [][]any{
		{"id", "resource.app.web", "connection_string", "postgres://admin@localhost:5432/main"},
	}, argsOf(log.withMessage("debug", "plugin=external provider=app create")))
}

func TestPluginExampleFailsWithoutExternalPlugin(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, "./does-not-exist")
	require.Error(t, err)
	require.Contains(t, err.Error(), "make build")
}

// eventPhases returns, for the events at level, the phases logged for each
// create, keyed by resource id
func eventPhases(t *testing.T, log *recordingLogger, level string) map[string][]string {
	t.Helper()

	phases := map[string][]string{}
	for _, m := range log.withMessage(level, "") {
		require.Equal(t, "event", m.args[0])
		require.Equal(t, "resource", m.args[2])

		if m.args[1] != "create" {
			continue
		}

		require.Equal(t, "phase", m.args[4])

		id := m.args[3].(string)
		phases[id] = append(phases[id], m.args[5].(string))
	}

	return phases
}

// TestPluginExampleLogsEventsAtDebug asserts every resource's events are
// logged at debug, a start and a success for a resource a provider creates,
// only a success for a builtin block, which has no provider
func TestPluginExampleLogsEventsAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	require.Equal(t, map[string][]string{
		"resource.postgres.main":                       {"start", "success"},
		"resource.postgres.replica":                    {"start", "success"},
		"module.analytics.resource.postgres.analytics": {"start", "success"},
		"resource.app.web":                             {"start", "success"},
		"module.analytics":                             {"success"},
		"module.analytics.output.location":             {"success"},
		"module.analytics.variable.db_username":        {"success"},
		"output.web_database":                          {"success"},
		"variable.db_password":                         {"success"},
		"variable.db_username":                         {"success"},
	}, eventPhases(t, log, "debug"))
}

// TestPluginExampleLogsPluginsLoadedAtDebug asserts loading each plugin, and
// the block types it provides, is logged at debug
func TestPluginExampleLogsPluginsLoadedAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	require.Equal(t, [][]any{{"block_types", "postgres"}}, argsOf(log.withMessage("debug", "plugin=ExamplePlugin plugin loaded")))
	require.Equal(t, [][]any{{"block_types", "app"}}, argsOf(log.withMessage("debug", "plugin=external plugin loaded")))
}

func TestPluginExampleLogsNoErrors(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	for _, m := range log.messages {
		require.NotEqual(t, "error", m.level, "unexpected error logged: %s %v", m.msg, m.args)
	}
}

// TestPluginExampleLogsNothingAtInfo asserts a successful run logs nothing
// above debug, only an error would stand out
func TestPluginExampleLogsNothingAtInfo(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	for _, m := range log.messages {
		require.Equal(t, "debug", m.level, "unexpected %s message: %s %v", m.level, m.msg, m.args)
	}
}

// TestPluginExampleLogsParseEventWithFileAtDebug asserts each resource's parse
// event is logged at debug with the file it was parsed from
func TestPluginExampleLogsParseEventWithFileAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, externalPlugin)
	require.NoError(t, err)

	files := map[string]string{}
	for _, m := range log.withMessage("debug", "") {
		if m.args[1] != "parse" {
			continue
		}

		require.Equal(t, []any{"event", "parse", "resource", m.args[3], "file", m.args[5], "phase", "success"}, m.args)
		files[m.args[3].(string)] = filepath.Base(m.args[5].(string))
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
