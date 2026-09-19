package xcl

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/stretchr/testify/require"
)

// eventRecorder records every event passed to its handler, Apply calls the
// handler from concurrent walk goroutines so recording is guarded by a mutex
type eventRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *eventRecorder) handle(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, e)
}

// find returns the events for the resource id, operation and phase
func (r *eventRecorder) find(id, operation, phase string) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := []Event{}
	for _, e := range r.events {
		if e.ResourceID == id && e.Operation == operation && e.Phase == phase {
			found = append(found, e)
		}
	}

	return found
}

// applyQueryFixtureWithEvents applies the query fixture, three registered
// database blocks and two plugin network blocks, with an event handler
func applyQueryFixtureWithEvents(t *testing.T) *eventRecorder {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := pr.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c := NewConfig(
		WithPluginRegistry(pr),
		WithEventHandler(recorder.handle),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return recorder
}

// TestApplyCallsEventHandlerWhenPluginResourceIsCreated asserts the handler
// receives the start and success of a plugin resource's create, with the
// serialized resource
func TestApplyCallsEventHandlerWhenPluginResourceIsCreated(t *testing.T) {
	recorder := applyQueryFixtureWithEvents(t)

	started := recorder.find("resource.network.frontend", "create", "start")
	require.Len(t, started, 1)
	require.Equal(t, "network.frontend", started[0].ResourceType)
	require.NotEmpty(t, started[0].Data)

	succeeded := recorder.find("resource.network.frontend", "create", "success")
	require.Len(t, succeeded, 1)
	require.NoError(t, succeeded[0].Error)
	require.NotEmpty(t, succeeded[0].Data)
}

// TestApplyCallsEventHandlerWhenRegisteredTypeIsApplied asserts the handler
// receives the success of a registered type, which has no provider and so no
// start event and no serialized resource
func TestApplyCallsEventHandlerWhenRegisteredTypeIsApplied(t *testing.T) {
	recorder := applyQueryFixtureWithEvents(t)

	require.Empty(t, recorder.find("resource.database.primary", "create", "start"))

	succeeded := recorder.find("resource.database.primary", "create", "success")
	require.Len(t, succeeded, 1)
	require.Equal(t, "database.primary", succeeded[0].ResourceType)
	require.Nil(t, succeeded[0].Data)
}

// TestApplyCallsEventHandlerWhenResourceIsParsed asserts the handler receives
// a parse event for each resource, with the file it was parsed from
func TestApplyCallsEventHandlerWhenResourceIsParsed(t *testing.T) {
	recorder := applyQueryFixtureWithEvents(t)

	file, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	parsed := recorder.find("resource.network.frontend", "parse", "success")
	require.Len(t, parsed, 1)
	require.Equal(t, "network.frontend", parsed[0].ResourceType)
	require.Equal(t, file, parsed[0].File)
}

// TestValidateCallsEventHandlerWhenResourceIsParsed asserts Validate passes
// the parse events to the handler, and nothing else as Validate creates
// nothing
func TestValidateCallsEventHandlerWhenResourceIsParsed(t *testing.T) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := pr.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c := NewConfig(
		WithPluginRegistry(pr),
		WithEventHandler(recorder.handle),
	)

	file, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Validate(file)
	require.NoError(t, err)

	require.Len(t, recorder.events, 5)
	for _, e := range recorder.events {
		require.Equal(t, "parse", e.Operation)
		require.Equal(t, "success", e.Phase)
		require.Equal(t, file, e.File)
	}
}
