package xcl

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// destroyFixture is everything a destroy test needs: a Config wired to a real
// file state store, the TestPlugin that records every provider call, the
// store and its file, the directory holding the configuration and the
// recorder receiving every lifecycle event
type destroyFixture struct {
	config     *Config
	plugin     *parser.TestPlugin
	store      *state.FileStateStore
	statePath  string
	configDir  string
	configFile string
	recorder   *eventRecorder
}

// setupDestroyConfig builds a Config with the TestPlugin registered, a file
// state store and an event recorder, and copies the dependent fixture into a
// temporary directory so a test can delete it. The fixture holds the chain
// network.first <- container.second <- container.third, network.independent,
// variable.independent_subnet and output.first_name.
func setupDestroyConfig(t *testing.T, log logger.Logger) *destroyFixture {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := registry.NewPluginRegistry(log)

	testPlugin := &parser.TestPlugin{}
	err := pr.RegisterPlugin(testPlugin)
	require.NoError(t, err)

	statePath := filepath.Join(t.TempDir(), "state.json")
	store, err := state.NewFileStateStore(statePath, pr)
	require.NoError(t, err)

	contents, err := os.ReadFile("./internal/test_fixtures/config/lifecycle/dependent/dependent.xcl")
	require.NoError(t, err)

	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "dependent.xcl")
	err = os.WriteFile(configFile, contents, 0644)
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(store),
		WithEventHandler(recorder.handle),
	)

	return &destroyFixture{
		config:     c,
		plugin:     testPlugin,
		store:      store,
		statePath:  statePath,
		configDir:  configDir,
		configFile: configFile,
		recorder:   recorder,
	}
}

// applyDestroyFixture applies the fixture's configuration, then clears the
// plugin's recorded calls and the recorder's events so a test sees only what
// the destroy does
func applyDestroyFixture(t *testing.T, f *destroyFixture) {
	t.Helper()

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	f.plugin.ResetCalls()

	f.recorder.mu.Lock()
	f.recorder.events = nil
	f.recorder.mu.Unlock()
}

// requireCalledBefore asserts first appears in calls before second
func requireCalledBefore(t *testing.T, calls []string, first, second string) {
	t.Helper()

	firstIndex := -1
	secondIndex := -1

	for i, c := range calls {
		if c == first && firstIndex == -1 {
			firstIndex = i
		}

		if c == second && secondIndex == -1 {
			secondIndex = i
		}
	}

	require.NotEqual(t, -1, firstIndex, "%s was not called, calls: %v", first, calls)
	require.NotEqual(t, -1, secondIndex, "%s was not called, calls: %v", second, calls)
	require.Less(t, firstIndex, secondIndex, "%s was not called before %s, calls: %v", first, second, calls)
}

// eventIndex returns the position of the first event matching id, operation
// and phase in the order the recorder received them, or -1
func eventIndex(r *eventRecorder, id, operation, phase string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, e := range r.events {
		if e.ResourceID == id && e.Operation == operation && e.Phase == phase {
			return i
		}
	}

	return -1
}

// destroyRecordingLogger records the level and message of everything logged
// to it, plugins may log from concurrent walk goroutines so recording is
// guarded by a mutex
type destroyRecordingLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *destroyRecordingLogger) record(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, level+": "+msg)
}

func (l *destroyRecordingLogger) Info(msg string, args ...any)  { l.record("info", msg) }
func (l *destroyRecordingLogger) Debug(msg string, args ...any) { l.record("debug", msg) }
func (l *destroyRecordingLogger) Warn(msg string, args ...any)  { l.record("warn", msg) }
func (l *destroyRecordingLogger) Error(msg string, args ...any) { l.record("error", msg) }

// aboveDebug returns every entry logged at info, warn or error
func (l *destroyRecordingLogger) aboveDebug() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	found := []string{}
	for _, e := range l.entries {
		if len(e) >= 7 && e[:7] == "debug: " {
			continue
		}

		found = append(found, e)
	}

	return found
}

// reset forgets everything logged so far
func (l *destroyRecordingLogger) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = nil
}

// TestConfigDestroyDestroysEverythingAndEmptiesState asserts destroying an
// applied configuration calls each provider-handled resource's destroy exactly
// once and leaves the saved and in-memory state empty
func TestConfigDestroyDestroysEverythingAndEmptiesState(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	err := f.config.Destroy()
	require.NoError(t, err)

	destroyed := f.plugin.GetDestroyedResources()
	require.Len(t, destroyed, 4)
	require.ElementsMatch(t, []string{
		"resource.network.first",
		"resource.container.second",
		"resource.container.third",
		"resource.network.independent",
	}, destroyed)

	saved, err := f.store.Load()
	require.NoError(t, err)
	require.Equal(t, 0, saved.ResourceCount())

	require.Equal(t, 0, f.config.ResourceCount())
}

// TestConfigDestroyNeedsNoConfiguration asserts destroy succeeds and destroys
// every created resource after the configuration files have been deleted
func TestConfigDestroyNeedsNoConfiguration(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	created := f.plugin.GetCreatedResources()
	require.Len(t, created, 4)

	f.plugin.ResetCalls()

	err = os.RemoveAll(f.configDir)
	require.NoError(t, err)

	err = f.config.Destroy()
	require.NoError(t, err)

	require.ElementsMatch(t, created, f.plugin.GetDestroyedResources())

	saved, err := f.store.Load()
	require.NoError(t, err)
	require.Equal(t, 0, saved.ResourceCount())
}

// TestConfigDestroyOrdersFromSavedStateWithoutConfiguration asserts that with
// the configuration deleted, destroy still removes children before their
// parents: third, then second, then first
func TestConfigDestroyOrdersFromSavedStateWithoutConfiguration(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	err := os.RemoveAll(f.configDir)
	require.NoError(t, err)

	err = f.config.Destroy()
	require.NoError(t, err)

	calls := f.plugin.GetCalls()
	requireCalledBefore(t, calls, "destroy resource.container.third", "destroy resource.container.second")
	requireCalledBefore(t, calls, "destroy resource.container.second", "destroy resource.network.first")
}

// TestConfigDestroyWithNothingSavedSucceeds asserts destroying when nothing
// was ever applied succeeds, calls no provider and never loads or saves state
func TestConfigDestroyWithNothingSavedSucceeds(t *testing.T) {
	c, testPlugin, ss := setupConfig(t)

	err := c.Destroy()
	require.NoError(t, err)

	ss.AssertNotCalled(t, "Load")
	ss.AssertNotCalled(t, "Save", mock.Anything)
	require.Empty(t, testPlugin.GetCalls())
}

// TestConfigDestroyWithNoStateStoreAndNothingAppliedSucceeds asserts a Config
// without a state store that never applied anything destroys nothing
func TestConfigDestroyWithNoStateStoreAndNothingAppliedSucceeds(t *testing.T) {
	pr := registry.NewPluginRegistry(logger.NewTestLogger(t))

	testPlugin := &parser.TestPlugin{}
	err := pr.RegisterPlugin(testPlugin)
	require.NoError(t, err)

	c := NewConfig(WithPluginRegistry(pr))

	err = c.Destroy()
	require.NoError(t, err)

	require.Empty(t, testPlugin.GetCalls())
}

// TestConfigDestroyWithEmptySavedStateWritesNothing asserts that when the
// saved state file exists but holds nothing, destroy succeeds, calls no
// provider and leaves the file as it was
func TestConfigDestroyWithEmptySavedStateWritesNothing(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))

	before, err := os.ReadFile(f.statePath)
	require.NoError(t, err)

	err = f.config.Destroy()
	require.NoError(t, err)

	require.Empty(t, f.plugin.GetCalls())

	after, err := os.ReadFile(f.statePath)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

// TestConfigDestroyReportsStartThenSuccess asserts a provider-handled resource
// reports its destroy starting and then succeeding
func TestConfigDestroyReportsStartThenSuccess(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	err := f.config.Destroy()
	require.NoError(t, err)

	started := f.recorder.find("resource.network.first", "destroy", "start")
	require.Len(t, started, 1)
	require.Equal(t, "network.first", started[0].ResourceType)
	require.NotEmpty(t, started[0].Data)

	succeeded := f.recorder.find("resource.network.first", "destroy", "success")
	require.Len(t, succeeded, 1)
	require.NoError(t, succeeded[0].Error)

	require.Empty(t, f.recorder.find("resource.network.first", "destroy", "error"))

	startIndex := eventIndex(f.recorder, "resource.network.first", "destroy", "start")
	successIndex := eventIndex(f.recorder, "resource.network.first", "destroy", "success")
	require.Less(t, startIndex, successIndex)
}

// TestConfigDestroyReportsStartThenErrorWhenDestroyFails asserts a resource
// whose provider destroy fails reports its destroy starting and then failing
func TestConfigDestroyReportsStartThenErrorWhenDestroyFails(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	f.plugin.SetDestroyError("resource.container.second", errors.New("boom"))

	err := f.config.Destroy()
	require.Error(t, err)

	started := f.recorder.find("resource.container.second", "destroy", "start")
	require.Len(t, started, 1)

	failed := f.recorder.find("resource.container.second", "destroy", "error")
	require.Len(t, failed, 1)
	require.ErrorContains(t, failed[0].Error, "boom")

	require.Empty(t, f.recorder.find("resource.container.second", "destroy", "success"))

	startIndex := eventIndex(f.recorder, "resource.container.second", "destroy", "start")
	errorIndex := eventIndex(f.recorder, "resource.container.second", "destroy", "error")
	require.Less(t, startIndex, errorIndex)
}

// TestConfigDestroyReportsOnlySuccessForVariable asserts a variable, which has
// no provider, reports only a destroy success and never a start
func TestConfigDestroyReportsOnlySuccessForVariable(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	err := f.config.Destroy()
	require.NoError(t, err)

	require.Empty(t, f.recorder.find("variable.independent_subnet", "destroy", "start"))

	succeeded := f.recorder.find("variable.independent_subnet", "destroy", "success")
	require.Len(t, succeeded, 1)
	require.Nil(t, succeeded[0].Data)
}

// TestConfigDestroyReturnsErrorNamingFailedResource asserts a failed destroy
// returns an error naming the resource, and the saved state keeps it marked
// destroy_failed along with its parent, which was never visited
func TestConfigDestroyReturnsErrorNamingFailedResource(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	f.plugin.SetDestroyError("resource.container.second", errors.New("boom"))

	err := f.config.Destroy()
	require.Error(t, err)
	require.ErrorContains(t, err, "destroy failed for resource.container.second")

	require.NotContains(t, f.plugin.GetDestroyedResources(), "resource.network.first")

	saved, err := f.store.Load()
	require.NoError(t, err)

	second, err := saved.FindResource("resource.container.second")
	require.NoError(t, err)

	secondMeta, err := types.GetMeta(second)
	require.NoError(t, err)
	require.Equal(t, types.StatusDestroyFailed, secondMeta.Status)

	_, err = saved.FindResource("resource.network.first")
	require.NoError(t, err)

	_, err = saved.FindResource("resource.container.third")
	require.Error(t, err)

	_, err = saved.FindResource("resource.network.independent")
	require.Error(t, err)

	require.Equal(t, 2, f.config.ResourceCount())
}

// TestConfigDestroyRetriesFailedResources asserts calling destroy again after
// a failure destroys what was left and empties the saved state
func TestConfigDestroyRetriesFailedResources(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	f.plugin.SetDestroyError("resource.container.second", errors.New("boom"))

	err := f.config.Destroy()
	require.Error(t, err)

	f.plugin.ClearErrors()
	f.plugin.ResetCalls()

	err = f.config.Destroy()
	require.NoError(t, err)

	require.ElementsMatch(t, []string{
		"resource.container.second",
		"resource.network.first",
	}, f.plugin.GetDestroyedResources())

	calls := f.plugin.GetCalls()
	requireCalledBefore(t, calls, "destroy resource.container.second", "destroy resource.network.first")

	saved, err := f.store.Load()
	require.NoError(t, err)
	require.Equal(t, 0, saved.ResourceCount())
	require.Equal(t, 0, f.config.ResourceCount())
}

// TestConfigDestroyFailsWhenSavedStateHasUnknownType asserts destroying with
// a registry that can not create a saved type fails with an UnknownTypesError,
// calls no provider and leaves the saved state untouched
func TestConfigDestroyFailsWhenSavedStateHasUnknownType(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	before, err := os.ReadFile(f.statePath)
	require.NoError(t, err)

	emptyRegistry := registry.NewPluginRegistry(logger.NewTestLogger(t))
	store, err := state.NewFileStateStore(f.statePath, emptyRegistry)
	require.NoError(t, err)

	c := NewConfig(
		WithPluginRegistry(emptyRegistry),
		WithStateStore(store),
	)

	err = c.Destroy()
	require.Error(t, err)

	var unknown state.UnknownTypesError
	require.True(t, errors.As(err, &unknown), "expected UnknownTypesError, got: %v", err)
	require.Contains(t, unknown.Types, "network")
	require.Contains(t, unknown.Types, "container")

	require.Empty(t, f.plugin.GetCalls())

	after, err := os.ReadFile(f.statePath)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

// TestConfigDestroyLogsNothingAboveDebug asserts a successful destroy logs
// nothing at info, warn or error through the registry's logger
func TestConfigDestroyLogsNothingAboveDebug(t *testing.T) {
	log := &destroyRecordingLogger{}

	f := setupDestroyConfig(t, log)
	applyDestroyFixture(t, f)

	log.reset()

	err := f.config.Destroy()
	require.NoError(t, err)

	require.Empty(t, log.aboveDebug())
}

func TestApplyRejectsEmptyConfigurationAndChangesNothing(t *testing.T) {
	f := setupDestroyConfig(t, logger.NewTestLogger(t))
	applyDestroyFixture(t, f)

	before, err := os.ReadFile(f.statePath)
	require.NoError(t, err)
	resourceCount := f.config.ResourceCount()
	require.Equal(t, 6, resourceCount)

	err = f.config.Apply("./internal/test_fixtures/config/empty")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEmptyConfiguration), "unexpected error: %v", err)

	require.Empty(t, f.plugin.GetCalls())

	after, err := os.ReadFile(f.statePath)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))

	require.Equal(t, resourceCount, f.config.ResourceCount())
}
