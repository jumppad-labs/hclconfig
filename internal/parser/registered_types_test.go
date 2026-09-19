package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser/mocks"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	registeredBasicConfig         = "../test_fixtures/config/registered/basic/main.xcl"
	registeredDisabledConfig      = "../test_fixtures/config/registered/disabled/main.xcl"
	registeredRemovedBeforeConfig = "../test_fixtures/config/registered/removed/before/main.xcl"
	registeredRemovedAfterConfig  = "../test_fixtures/config/registered/removed/after/main.xcl"

	registeredVariableID       = "variable.environment"
	registeredDatabaseID       = "resource.database.main"
	registeredAppID            = "resource.app.web"
	registeredConsumerID       = "resource.consumer.reader"
	registeredModuleDatabaseID = "module.shared.resource.database.shared"
	registeredDisabledID       = "resource.database.off"
	registeredKeptID           = "resource.database.kept"
	registeredRemovedID        = "resource.database.removed"
)

// registeredHarness holds what every apply in a registered type scenario
// shares: one plugin registry with the registered fixture types and NO
// plugins, and one file state store. Each apply builds a fresh Parser from
// these, just as separate runs would.
type registeredHarness struct {
	registry *registry.PluginRegistry
	store    *state.FileStateStore
}

// setupRegisteredTypes redirects HOME to a temp dir, registers the database,
// app and consumer types into a new registry without registering any plugin,
// and creates a file state store in a temp dir.
func setupRegisteredTypes(t *testing.T) *registeredHarness {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeApp, &registered.App{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeConsumer, &registered.Consumer{})
	require.NoError(t, err)

	store, err := state.NewFileStateStore(filepath.Join(t.TempDir(), "state.json"), reg)
	require.NoError(t, err)

	return &registeredHarness{
		registry: reg,
		store:    store,
	}
}

// newParser builds a fresh Parser that shares the harness registry and state
// store. log and onEvent may be nil.
func (h *registeredHarness) newParser(t *testing.T, log logger.Logger, onEvent func(ParserEvent)) *Parser {
	t.Helper()

	options := testOptions(t)
	if log != nil {
		options.Logger = log
	}

	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.OnParserEvent = onEvent

	return NewParser(options)
}

// applyAndSave applies the config at path with a fresh Parser, requires it to
// succeed, saves the returned state as Config.Apply does and returns it.
func (h *registeredHarness) applyAndSave(t *testing.T, path string) *state.State {
	t.Helper()

	p := h.newParser(t, nil, nil)

	st, err := p.Apply(path)
	require.NoError(t, err)
	require.NotNil(t, st)

	err = h.store.Save(st)
	require.NoError(t, err)

	return st
}

// requireMeta returns the Meta of the resource with the given ID in st
func requireMeta(t *testing.T, st *state.State, id string) *types.Meta {
	t.Helper()

	r, err := st.FindResource(id)
	require.NoError(t, err)

	meta, err := types.GetMeta(r)
	require.NoError(t, err)

	return meta
}

// requireDatabase returns the resource with the given ID in st, requiring it
// to be exactly a *registered.Database
func requireDatabase(t *testing.T, st *state.State, id string) *registered.Database {
	t.Helper()

	r, err := st.FindResource(id)
	require.NoError(t, err)

	db, ok := r.(*registered.Database)
	require.True(t, ok, "expected *registered.Database, got %T", r)

	return db
}

func TestApplyRegisteredTypesSucceedsWithoutProvider(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	variableStatus := requireMeta(t, st, registeredVariableID).Status

	require.Equal(t, variableStatus, requireMeta(t, st, registeredDatabaseID).Status)
	require.Equal(t, variableStatus, requireMeta(t, st, registeredAppID).Status)
	require.Equal(t, variableStatus, requireMeta(t, st, registeredConsumerID).Status)
	require.Equal(t, variableStatus, requireMeta(t, st, registeredModuleDatabaseID).Status)
}

func TestApplyRegisteredTypeDecodesValuesAndNestedBlock(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	db := requireDatabase(t, st, registeredDatabaseID)

	require.Equal(t, "us-east", db.Location)
	require.Equal(t, 5432, db.Port)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 30, db.Timeouts.Connect)
	require.Equal(t, 60, db.Timeouts.Read)
}

func TestApplyRegisteredTypeReturnsRegisteredGoType(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := st.FindResource(registeredDatabaseID)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(&registered.Database{}), reflect.TypeOf(r))

	_, ok := r.(*registered.Database)
	require.True(t, ok)

	r, err = st.FindResource(registeredAppID)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(&registered.App{}), reflect.TypeOf(r))

	r, err = st.FindResource(registeredConsumerID)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(&registered.Consumer{}), reflect.TypeOf(r))
}

func TestApplyRegisteredTypeResolvesReferences(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := st.FindResource(registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok)

	require.Equal(t, "production", app.Environment)
	require.Equal(t, "us-east", app.DatabaseLocation)
	require.Equal(t, 5432, app.DatabasePort)

	r, err = st.FindResource(registeredConsumerID)
	require.NoError(t, err)
	consumer, ok := r.(*registered.Consumer)
	require.True(t, ok)

	require.Equal(t, "production", consumer.AppEnvironment)
}

func TestApplyRegisteredTypeReadsModuleOutput(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := st.FindResource(registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok)

	require.Equal(t, "eu-west", app.SharedLocation)
}

func TestApplyProcessesDependentAfterRegisteredType(t *testing.T) {
	h := setupRegisteredTypes(t)

	// events fire from the walker's parallel goroutines
	var mu sync.Mutex
	created := []string{}
	onEvent := func(e ParserEvent) {
		if e.Operation != "create" || e.Phase != "success" {
			return
		}

		mu.Lock()
		defer mu.Unlock()
		created = append(created, e.ResourceID)
	}

	p := h.newParser(t, nil, onEvent)

	_, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	requireBefore(t, registeredDatabaseID, registeredAppID, created)
	requireBefore(t, registeredAppID, registeredConsumerID, created)
}

func TestReapplyRegisteredTypesSucceedsWithoutProvider(t *testing.T) {
	h := setupRegisteredTypes(t)

	h.applyAndSave(t, registeredBasicConfig)

	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	db := requireDatabase(t, st, registeredDatabaseID)
	require.Equal(t, "us-east", db.Location)
	require.Equal(t, 5432, db.Port)
}

func TestApplyAfterRemovingRegisteredBlockSucceedsWithoutProvider(t *testing.T) {
	h := setupRegisteredTypes(t)

	first := h.applyAndSave(t, registeredRemovedBeforeConfig)
	requireDatabase(t, first, registeredRemovedID)

	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredRemovedAfterConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	requireDatabase(t, st, registeredKeptID)

	_, err = st.FindResource(registeredRemovedID)
	require.Error(t, err)
}

func TestApplyRegisteredTypeInModuleIsStoredUnderModulePath(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	db := requireDatabase(t, st, registeredModuleDatabaseID)

	require.Equal(t, registeredModuleDatabaseID, db.Meta.ID)
	require.Equal(t, "shared", db.Meta.Module)
	require.Equal(t, "eu-west", db.Location)
	require.Equal(t, 5433, db.Port)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 10, db.Timeouts.Connect)

	moduleResources, err := st.FindModuleResources("module.shared", false)
	require.NoError(t, err)
	require.Contains(t, moduleResources, any(db))
}

func TestApplyMarksDisabledRegisteredTypeDisabled(t *testing.T) {
	h := setupRegisteredTypes(t)

	var mu sync.Mutex
	eventIDs := []string{}
	onEvent := func(e ParserEvent) {
		// a disabled resource is still parsed, only the walk skips it
		if e.Operation == "parse" {
			return
		}

		mu.Lock()
		defer mu.Unlock()
		eventIDs = append(eventIDs, e.ResourceID)
	}

	p := h.newParser(t, nil, onEvent)

	st, err := p.Apply(registeredDisabledConfig)
	require.NoError(t, err)

	r, err := st.FindResource(registeredDisabledID)
	require.NoError(t, err)

	disabled, err := types.GetDisabled(r)
	require.NoError(t, err)
	require.True(t, disabled)

	meta, err := types.GetMeta(r)
	require.NoError(t, err)
	require.Empty(t, meta.Status)

	// a disabled resource is skipped by the walk, no lifecycle event fires for it
	mu.Lock()
	defer mu.Unlock()
	require.NotContains(t, eventIDs, registeredDisabledID)
}

func TestApplyRegisteredTypeWithComputedFieldLogsNoWarning(t *testing.T) {
	h := setupRegisteredTypes(t)

	log := &recordingLogger{}
	p := h.newParser(t, log, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	require.Empty(t, log.warnings())

	db := requireDatabase(t, st, registeredDatabaseID)
	require.Equal(t, "", db.ConnectionString)
}

func TestRegisteredResourcesAreInStateAndFoundByPath(t *testing.T) {
	h := setupRegisteredTypes(t)

	h.applyAndSave(t, registeredBasicConfig)

	// load the state saved by the apply, this round trips it through json
	loaded, err := h.store.Load()
	require.NoError(t, err)

	db := requireDatabase(t, loaded, registeredDatabaseID)
	require.Equal(t, "us-east", db.Location)
	require.Equal(t, 5432, db.Port)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 30, db.Timeouts.Connect)
	require.Equal(t, 60, db.Timeouts.Read)

	moduleDB := requireDatabase(t, loaded, registeredModuleDatabaseID)
	require.Equal(t, "eu-west", moduleDB.Location)

	r, err := loaded.FindResource(registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok, "expected *registered.App, got %T", r)
	require.Equal(t, "production", app.Environment)
	require.Equal(t, "us-east", app.DatabaseLocation)
	require.Equal(t, 5432, app.DatabasePort)
	require.Equal(t, "eu-west", app.SharedLocation)

	r, err = loaded.FindResource(registeredConsumerID)
	require.NoError(t, err)
	consumer, ok := r.(*registered.Consumer)
	require.True(t, ok, "expected *registered.Consumer, got %T", r)
	require.Equal(t, "production", consumer.AppEnvironment)
	require.Equal(t, []string{registeredAppID}, consumer.DependsOn)
}

func TestDestroyWalkSkipsProviderForRegisteredType(t *testing.T) {
	h := setupRegisteredTypes(t)

	// no expectations, any provider lookup fails the test
	resolver := mocks.NewMockProviderResolver(t)

	db := &registered.Database{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   registeredDatabaseID,
				Name: "main",
				Type: registered.TypeDatabase,
			},
		},
		Location: "us-east",
		Port:     5432,
	}

	callback := destroyWalkCallback(resolver, h.registry, testOptions(t))

	diags := callback(db)
	require.False(t, diags.HasErrors())
	require.Empty(t, diags)
	require.Equal(t, types.StatusDestroyed, db.Meta.Status)
}
