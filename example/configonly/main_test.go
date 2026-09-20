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

	"github.com/jumppad-labs/xcl/example/configonly/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// configDir is the configuration this example parses
const configDir = "./config"

// declaredResourceIDs is every resource the configuration declares, sorted
var declaredResourceIDs = []string{
	"output.api_url",
	"resource.config_map.api",
	"resource.deployment.api",
	"resource.ingress.api",
	"resource.service.api",
	"variable.image_tag",
	"variable.replicas",
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

// deployment runs the example and returns the parsed deployment, the
// resource most of these tests are about
func deployment(t *testing.T) *resources.Deployment {
	t.Helper()

	found, err := run(&bytes.Buffer{}, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	d, ok := findResource(t, found, "resource.deployment.api").(*resources.Deployment)
	require.True(t, ok)

	return d
}

func TestConfigOnlyExampleFindsDeclaredResources(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

// TestConfigOnlyExampleReturnsRegisteredGoTypes asserts each block is decoded
// into the Go type that was registered for it, a registered type is held as
// itself rather than a type generated from a schema
func TestConfigOnlyExampleReturnsRegisteredGoTypes(t *testing.T) {
	found, err := run(&bytes.Buffer{}, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	_, ok := findResource(t, found, "resource.config_map.api").(*resources.ConfigMap)
	require.True(t, ok)

	_, ok = findResource(t, found, "resource.deployment.api").(*resources.Deployment)
	require.True(t, ok)

	_, ok = findResource(t, found, "resource.service.api").(*resources.Service)
	require.True(t, ok)

	_, ok = findResource(t, found, "resource.ingress.api").(*resources.Ingress)
	require.True(t, ok)
}

// TestConfigOnlyExampleDecodesRepeatedBlocks asserts a block that appears
// more than once is decoded into a slice, in the order it was written
func TestConfigOnlyExampleDecodesRepeatedBlocks(t *testing.T) {
	d := deployment(t)

	require.Len(t, d.Containers, 2)
	require.Equal(t, "api", d.Containers[0].Name)
	require.Equal(t, "proxy", d.Containers[1].Name)

	require.Len(t, d.Containers[0].Ports, 2)
	require.Equal(t, "http", d.Containers[0].Ports[0].Name)
	require.Equal(t, 8080, d.Containers[0].Ports[0].ContainerPort)
	require.Equal(t, "metrics", d.Containers[0].Ports[1].Name)
	require.Equal(t, 9090, d.Containers[0].Ports[1].ContainerPort)

	require.Len(t, d.Containers[1].Ports, 1)
	require.Equal(t, 8081, d.Containers[1].Ports[0].ContainerPort)
}

// TestConfigOnlyExampleDecodesNestedBlocks asserts blocks nested inside a
// nested block are decoded, resources holds limits and requests
func TestConfigOnlyExampleDecodesNestedBlocks(t *testing.T) {
	d := deployment(t)

	api := d.Containers[0]
	require.NotNil(t, api.Resources)
	require.Equal(t, "500m", api.Resources.Limits.CPU)
	require.Equal(t, "512Mi", api.Resources.Limits.Memory)
	require.Equal(t, "100m", api.Resources.Requests.CPU)
	require.Equal(t, "128Mi", api.Resources.Requests.Memory)

	require.Len(t, api.VolumeMounts, 1)
	require.Equal(t, "config", api.VolumeMounts[0].Name)
	require.Equal(t, "/etc/api", api.VolumeMounts[0].Path)

	require.Len(t, d.Volumes, 1)
	require.Equal(t, "config", d.Volumes[0].Name)
}

// TestConfigOnlyExampleLeavesOmittedBlockNil asserts a block the
// configuration leaves out is nil, the proxy container sets no resources
func TestConfigOnlyExampleLeavesOmittedBlockNil(t *testing.T) {
	d := deployment(t)

	require.Nil(t, d.Containers[1].Resources)
	require.Empty(t, d.Containers[1].Env)
	require.Empty(t, d.Containers[1].VolumeMounts)
}

// TestConfigOnlyExampleReadsValuesFromConfigMap asserts values read out of a
// map attribute of another resource are resolved
func TestConfigOnlyExampleReadsValuesFromConfigMap(t *testing.T) {
	d := deployment(t)

	env := d.Containers[0].Env
	require.Len(t, env, 2)
	require.Equal(t, "DB_HOST", env[0].Name)
	require.Equal(t, "postgres.default.svc", env[0].Value)
	require.Equal(t, "LOG_LEVEL", env[1].Name)
	require.Equal(t, "info", env[1].Value)

	require.Equal(t, "resource.config_map.api", d.Volumes[0].ConfigMap)
}

// TestConfigOnlyExampleReadsVariables asserts a variable is read both as a
// value of its own and inside an interpolated string
func TestConfigOnlyExampleReadsVariables(t *testing.T) {
	d := deployment(t)

	require.Equal(t, 3, d.Replicas)
	require.Equal(t, "ghcr.io/example/api:1.2.0", d.Containers[0].Image)
}

// TestConfigOnlyExampleLinksServiceToDeployment asserts the service names the
// deployment by id and reads its target port out of it, the port coming from
// a repeated block referenced by position
func TestConfigOnlyExampleLinksServiceToDeployment(t *testing.T) {
	found, err := run(&bytes.Buffer{}, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	service, ok := findResource(t, found, "resource.service.api").(*resources.Service)
	require.True(t, ok)

	require.Equal(t, "resource.deployment.api", service.Deployment)
	require.Equal(t, 80, service.Port)
	require.Equal(t, 8080, service.TargetPort)
}

// TestConfigOnlyExampleLinksIngressToService asserts the ingress rule names
// the service it routes to by id, and reads its port
func TestConfigOnlyExampleLinksIngressToService(t *testing.T) {
	found, err := run(&bytes.Buffer{}, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	ingress, ok := findResource(t, found, "resource.ingress.api").(*resources.Ingress)
	require.True(t, ok)

	require.Equal(t, "api.example.com", ingress.Host)
	require.Len(t, ingress.Rules, 1)
	require.Equal(t, "/", ingress.Rules[0].Path)
	require.Equal(t, "resource.service.api", ingress.Rules[0].Service)
	require.Equal(t, 80, ingress.Rules[0].Port)
}

func TestConfigOnlyExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

// TestConfigOnlyExamplePrintsNestedBlocks asserts the printed deployment
// walks the blocks nested inside it
func TestConfigOnlyExamplePrintsNestedBlocks(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Deployments\n")
	require.Contains(t, out.String(), "  resource.deployment.api replicas=3\n")
	require.Contains(t, out.String(), "    container api image=ghcr.io/example/api:1.2.0\n")
	require.Contains(t, out.String(), "      port http container_port=8080\n")
	require.Contains(t, out.String(), "      env DB_HOST=postgres.default.svc\n")
	require.Contains(t, out.String(), "      limits cpu=500m memory=512Mi\n")
	require.Contains(t, out.String(), "      requests cpu=100m memory=128Mi\n")
	require.Contains(t, out.String(), "      volume_mount config path=/etc/api\n")
	require.Contains(t, out.String(), "    volume config config_map=resource.config_map.api\n")
	require.Contains(t, out.String(), "    container proxy image=ghcr.io/example/proxy:0.4.1\n")
}

// TestConfigOnlyExamplePrintsLinkedResources asserts the printed service and
// ingress hold the values they read from the blocks they reference
func TestConfigOnlyExamplePrintsLinkedResources(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Service\n")
	require.Contains(t, out.String(), "  resource.service.api deployment=resource.deployment.api port=80 target_port=8080\n")
	require.Contains(t, out.String(), "## Ingress\n")
	require.Contains(t, out.String(), "  resource.ingress.api host=api.example.com\n")
	require.Contains(t, out.String(), "    rule path=/ service=resource.service.api port=80\n")
}

func TestConfigOnlyExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), "./does-not-exist", filepath.Join(t.TempDir(), "state.json"))
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

	_, err := run(&bytes.Buffer{}, log, configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	files := map[string]string{}
	for _, args := range log.events("debug", "parse") {
		require.Equal(t, []any{"event", "parse", "resource", args[3], "file", args[5], "phase", "success"}, args)
		files[args[3].(string)] = filepath.Base(args[5].(string))
	}

	require.Equal(t, map[string]string{
		"variable.image_tag":      "main.xcl",
		"variable.replicas":       "main.xcl",
		"resource.config_map.api": "main.xcl",
		"resource.deployment.api": "main.xcl",
		"resource.service.api":    "main.xcl",
		"resource.ingress.api":    "main.xcl",
		"output.api_url":          "main.xcl",
	}, files)
}

// TestConfigOnlyExampleLogsCreateSuccessWithoutStartAtDebug asserts every
// resource's create is logged at debug as a success only, registered types
// have no provider so there is nothing to start
func TestConfigOnlyExampleLogsCreateSuccessWithoutStartAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, filepath.Join(t.TempDir(), "state.json"))
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

	_, err := run(&bytes.Buffer{}, log, configDir, filepath.Join(t.TempDir(), "state.json"))
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

	_, err = run(&bytes.Buffer{}, log, dir, filepath.Join(t.TempDir(), "state.json"))
	require.Error(t, err)

	failed := log.events("error", "parse")
	require.Len(t, failed, 1)
	require.Equal(t, "resource.nosuchtype.broken", failed[0][3])
	require.Equal(t, file, failed[0][5])
}

// TestConfigOnlyExampleDestroysEverythingItApplied asserts the state saved after a run is
// empty, everything that was applied has been destroyed
func TestConfigOnlyExampleDestroysEverythingItApplied(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	_, err := run(&bytes.Buffer{}, logger.NewTestLogger(t), configDir, statePath)
	require.NoError(t, err)

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))
	require.NoError(t, reg.RegisterType("config_map", &resources.ConfigMap{}))
	require.NoError(t, reg.RegisterType("deployment", &resources.Deployment{}))
	require.NoError(t, reg.RegisterType("service", &resources.Service{}))
	require.NoError(t, reg.RegisterType("ingress", &resources.Ingress{}))

	store, err := state.NewFileStateStore(statePath, reg)
	require.NoError(t, err)

	saved, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, 0, saved.ResourceCount())
}

func TestConfigOnlyExamplePrintsNoResourcesRemaining(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, logger.NewTestLogger(t), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Destroyed\n  0 resources remaining\n")
}

// TestConfigOnlyExampleLogsDestroySuccessWithoutStartAtDebug asserts every
// applied resource's destroy is logged at debug as a success only, registered
// types have no provider so there is nothing to start
func TestConfigOnlyExampleLogsDestroySuccessWithoutStartAtDebug(t *testing.T) {
	log := &recordingLogger{}

	_, err := run(&bytes.Buffer{}, log, configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	ids := []string{}
	for _, args := range log.events("debug", "destroy") {
		require.Equal(t, "success", args[5])
		ids = append(ids, args[3].(string))
	}
	sort.Strings(ids)

	require.Equal(t, declaredResourceIDs, ids)
}
