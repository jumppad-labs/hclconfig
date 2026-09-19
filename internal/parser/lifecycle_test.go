package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	lifecycleOriginalConfig = "../test_fixtures/config/lifecycle/original/single.xcl"
	lifecycleEditedConfig   = "../test_fixtures/config/lifecycle/edited/single.xcl"
	lifecycleMovedConfig    = "../test_fixtures/config/lifecycle/moved/network.xcl"

	lifecycleDependentConfig = "../test_fixtures/config/lifecycle/dependent/dependent.xcl"

	lifecycleComputedRefConfig = "../test_fixtures/config/lifecycle/computed_ref/computed_ref.xcl"
	lifecycleNestedConfig      = "../test_fixtures/config/lifecycle/nested/nested.xcl"
	lifecycleReorderedOriginal = "../test_fixtures/config/lifecycle/reordered/original/web.xcl"
	lifecycleReorderedSwapped  = "../test_fixtures/config/lifecycle/reordered/swapped/web.xcl"
	lifecycleReferenceConfig   = "../test_fixtures/config/lifecycle/reference/reference.xcl"

	lifecycleNetworkID = "resource.network.one"

	computedRefUserID    = "resource.container.user"
	nestedWebID          = "resource.container.web"
	nestedConsumerID     = "resource.container.consumer"
	referenceSourceID    = "resource.network.a"
	referenceReferenceID = "resource.network.b"

	changedConfiguredValueWarning = "provider changed a configured value"

	dependentFirstID       = "resource.network.first"
	dependentSecondID      = "resource.container.second"
	dependentThirdID       = "resource.container.third"
	dependentIndependentID = "resource.network.independent"
)

// lifecycleHarness holds what every apply in a lifecycle scenario shares: one
// plugin registry, one TestPlugin registered into it and one file state store.
// Each apply builds a fresh Parser from these, just as separate runs would.
type lifecycleHarness struct {
	registry  *registry.PluginRegistry
	plugin    *TestPlugin
	store     *state.FileStateStore
	statePath string

	// log, when set, is the logger every parser built by newParser uses in
	// place of a test logger
	log logger.Logger
}

// setupLifecycle redirects HOME to a temp dir, registers a TestPlugin into a new
// registry and creates a file state store in a temp dir.
func setupLifecycle(t *testing.T) *lifecycleHarness {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	testPlugin := &TestPlugin{}
	err := reg.RegisterPlugin(testPlugin)
	require.NoError(t, err)

	statePath := filepath.Join(t.TempDir(), "state.json")
	store, err := state.NewFileStateStore(statePath, reg)
	require.NoError(t, err)

	return &lifecycleHarness{
		registry:  reg,
		plugin:    testPlugin,
		store:     store,
		statePath: statePath,
	}
}

// newParser builds a fresh Parser that shares the harness registry and state
// store. onEvent may be nil.
func (h *lifecycleHarness) newParser(t *testing.T, onEvent func(ParserEvent)) *Parser {
	t.Helper()

	options := testOptions(t)
	options.Logger = logger.NewTestLogger(t)
	if h.log != nil {
		options.Logger = h.log
	}

	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.OnParserEvent = onEvent

	return NewParser(options)
}

// applyAndSave applies the config at path with a fresh Parser, requires it to
// succeed, saves the returned state and returns it.
func (h *lifecycleHarness) applyAndSave(t *testing.T, path string) *state.State {
	t.Helper()

	p := h.newParser(t, nil)

	st, err := p.Apply(path)
	require.NoError(t, err)
	require.NotNil(t, st)

	err = h.store.Save(st)
	require.NoError(t, err)

	return st
}

// applyAndSaveExpectingFailure applies the config at path with a fresh Parser
// and requires it to fail. Like Config.Apply it saves whatever state the failed
// apply returned, then returns that state and the error.
func (h *lifecycleHarness) applyAndSaveExpectingFailure(t *testing.T, path string) (*state.State, error) {
	t.Helper()

	p := h.newParser(t, nil)

	st, err := p.Apply(path)
	require.Error(t, err)

	if st != nil {
		saveErr := h.store.Save(st)
		require.NoError(t, saveErr)
	}

	return st, err
}

// loadSaved loads the state the harness store last saved.
func (h *lifecycleHarness) loadSaved(t *testing.T) *state.State {
	t.Helper()

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.NotNil(t, saved)

	return saved
}

// resourceStatus returns the status recorded for the resource in the given state.
func resourceStatus(t *testing.T, st *state.State, resourceID string) string {
	t.Helper()

	resource, err := st.FindResource(resourceID)
	require.NoError(t, err)

	meta, err := types.GetMeta(resource)
	require.NoError(t, err)

	return meta.Status
}

// callsFor returns the plugin calls made for the resource, in order.
func callsFor(calls []string, resourceID string) []string {
	matching := []string{}
	for _, call := range calls {
		if strings.HasSuffix(call, " "+resourceID) {
			matching = append(matching, call)
		}
	}

	return matching
}

// eventCollector gathers parser events; the walker fires them in parallel.
// eventCollector records the lifecycle events of an apply. Parse events, fired
// as each block is read, are not recorded, they have tests of their own.
type eventCollector struct {
	mu     sync.Mutex
	events []ParserEvent
}

func (c *eventCollector) collect(event ParserEvent) {
	if event.Operation == "parse" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *eventCollector) all() []ParserEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ParserEvent{}, c.events...)
}

// networkStatus returns the status recorded for the network in the given state.
func networkStatus(t *testing.T, st *state.State) string {
	t.Helper()

	resource, err := st.FindResource(lifecycleNetworkID)
	require.NoError(t, err)

	meta, err := types.GetMeta(resource)
	require.NoError(t, err)

	return meta.Status
}

// eventsFor returns the "<operation> <phase>" of each event for the resource,
// in the order they fired.
func eventsFor(events []ParserEvent, resourceID string) []string {
	fired := []string{}
	for _, event := range events {
		if event.ResourceID == resourceID {
			fired = append(fired, fmt.Sprintf("%s %s", event.Operation, event.Phase))
		}
	}

	return fired
}

func TestApplyUnchangedConfigTwiceCreatesOnceAndOnlyReadsOnSecondApply(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetCreatedResources())

	h.plugin.ResetCalls()

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetReadResources())
	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Equal(t, types.StatusCreated, networkStatus(t, st))

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.Equal(t, types.StatusCreated, networkStatus(t, saved))
}

func TestFirstApplyNeverReads(t *testing.T) {
	h := setupLifecycle(t)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Empty(t, h.plugin.GetReadResources())
	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetCreatedResources())
	require.Equal(t, []string{"create " + lifecycleNetworkID}, h.plugin.GetCalls())
	require.Equal(t, types.StatusCreated, networkStatus(t, st))
}

func TestReadReceivesSavedAndConfiguredCopies(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	h.applyAndSave(t, lifecycleOriginalConfig)

	readCalls := h.plugin.GetReadCalls()
	require.Len(t, readCalls, 1)
	require.Equal(t, lifecycleNetworkID, readCalls[0].ID)

	// the saved copy carries what Create set and the status it was saved with
	saved := map[string]any{}
	err := json.Unmarshal(readCalls[0].Old, &saved)
	require.NoError(t, err)
	require.Equal(t, "id-one", saved["provider_id"])
	require.Equal(t, "10.0.0.0/16", saved["subnet"])

	savedMeta, ok := saved["meta"].(map[string]any)
	require.True(t, ok, "saved copy has no meta: %s", string(readCalls[0].Old))
	require.Equal(t, types.StatusCreated, savedMeta["status"])

	// the configured copy holds the configured subnet, and the computed
	// provider id saved last time has already been carried onto it before the
	// read, so a provider with no custom read still keeps it
	configured := map[string]any{}
	err = json.Unmarshal(readCalls[0].New, &configured)
	require.NoError(t, err)
	require.Equal(t, "10.0.0.0/16", configured["subnet"])
	require.Equal(t, "id-one", configured["provider_id"])
}

func TestConfigEditTriggersUpdate(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	st := h.applyAndSave(t, lifecycleEditedConfig)

	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetCreatedResources())

	network := findResource[structs.Network](t, st, lifecycleNetworkID)
	require.Equal(t, "10.1.0.0/16", network.Subnet)
	require.Equal(t, types.StatusUpdated, networkStatus(t, st))
}

func TestDriftTriggersUpdate(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetReadObserved(lifecycleNetworkID, "stopped")

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetCreatedResources())
	require.Equal(t, types.StatusUpdated, networkStatus(t, st))
}

func TestMissingResourceIsRecreated(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetReadNotFound(lifecycleNetworkID)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetReadResources())
	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Equal(t, types.StatusCreated, networkStatus(t, st))
}

func TestReadFailureFailsApply(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetReadError(lifecycleNetworkID, fmt.Errorf("network API unavailable"))

	p := h.newParser(t, nil)
	st, err := p.Apply(lifecycleOriginalConfig)

	require.Error(t, err)
	require.NotNil(t, st)
	require.Contains(t, err.Error(), "read failed for "+lifecycleNetworkID)
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetCreatedResources())
}

func TestDefaultChangeDetectionNoUpdateWhenNothingDiffers(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{
		"read " + lifecycleNetworkID,
		"changed " + lifecycleNetworkID,
	}, h.plugin.GetCalls())
	require.Empty(t, h.plugin.GetUpdatedResources())
}

func TestDefaultChangeDetectionUpdatesWhenValueDiffers(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	h.applyAndSave(t, lifecycleEditedConfig)

	require.Equal(t, []string{
		"read " + lifecycleNetworkID,
		"changed " + lifecycleNetworkID,
		"update " + lifecycleNetworkID,
	}, h.plugin.GetCalls())
}

func TestDefaultChangeDetectionIgnoresMetadata(t *testing.T) {
	h := setupLifecycle(t)

	first := h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	second := h.applyAndSave(t, lifecycleMovedConfig)

	// the metadata really does differ between the two applies
	firstNetwork := findResource[structs.Network](t, first, lifecycleNetworkID)
	secondNetwork := findResource[structs.Network](t, second, lifecycleNetworkID)
	require.NotEqual(t, firstNetwork.Meta.File, secondNetwork.Meta.File)
	require.NotEqual(t, firstNetwork.Meta.Line, secondNetwork.Meta.Line)

	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetReadResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetCreatedResources())
}

func TestOverriddenChangeDetectionIsUsed(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetChangedResult(lifecycleNetworkID, false)

	h.applyAndSave(t, lifecycleEditedConfig)

	// the edited subnet would be a change by default, the override says it is not
	require.Equal(t, []string{
		"read " + lifecycleNetworkID,
		"changed " + lifecycleNetworkID,
	}, h.plugin.GetCalls())
	require.Empty(t, h.plugin.GetUpdatedResources())
}

func TestOverriddenChangeDetectionCanReportAChange(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetChangedResult(lifecycleNetworkID, true)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{lifecycleNetworkID}, h.plugin.GetUpdatedResources())
	require.Equal(t, types.StatusUpdated, networkStatus(t, st))
}

func TestUnchangedResourceSavesWhatWasRead(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetReadObserved(lifecycleNetworkID, "running")
	h.plugin.SetChangedResult(lifecycleNetworkID, false)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Empty(t, h.plugin.GetUpdatedResources())

	network := findResource[structs.Network](t, st, lifecycleNetworkID)
	require.Equal(t, "running", network.Observed)
	require.Equal(t, types.StatusCreated, network.Meta.Status)
}

func TestUnchangedResourceKeepsUpdatedStatus(t *testing.T) {
	h := setupLifecycle(t)

	first := h.applyAndSave(t, lifecycleOriginalConfig)
	require.Equal(t, types.StatusCreated, networkStatus(t, first))

	second := h.applyAndSave(t, lifecycleEditedConfig)
	require.Equal(t, types.StatusUpdated, networkStatus(t, second))

	h.plugin.ResetCalls()

	third := h.applyAndSave(t, lifecycleEditedConfig)

	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Equal(t, types.StatusUpdated, networkStatus(t, third))
}

func TestReadEventsUseReadOperation(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Apply(lifecycleOriginalConfig)
	require.NoError(t, err)

	events := collector.all()
	requireEvent(t, events, "read", "start", lifecycleNetworkID)
	requireEvent(t, events, "read", "success", lifecycleNetworkID)

	require.Equal(t, []string{
		"read start",
		"read success",
		"changed start",
		"changed success",
	}, eventsFor(events, lifecycleNetworkID))

	for _, event := range events {
		require.NotEqual(t, "refresh", event.Operation)
	}
}

func TestReadEventsReportReadError(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetReadError(lifecycleNetworkID, fmt.Errorf("network API unavailable"))

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Apply(lifecycleOriginalConfig)
	require.Error(t, err)

	events := collector.all()
	errorEvent := requireEvent(t, events, "read", "error", lifecycleNetworkID)
	require.ErrorContains(t, errorEvent.Error, "network API unavailable")

	require.Equal(t, []string{
		"read start",
		"read error",
	}, eventsFor(events, lifecycleNetworkID))

	for _, event := range events {
		require.NotEqual(t, "refresh", event.Operation)
	}
}

func TestFailureSkipsOnlyDependents(t *testing.T) {
	h := setupLifecycle(t)
	h.plugin.SetCreateError(dependentFirstID, fmt.Errorf("network API unavailable"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	require.Contains(t, err.Error(), "create failed for "+dependentFirstID)

	calls := h.plugin.GetCalls()
	require.Equal(t, []string{"create " + dependentFirstID}, callsFor(calls, dependentFirstID))
	require.Empty(t, callsFor(calls, dependentSecondID))
	require.Empty(t, callsFor(calls, dependentThirdID))
	require.Equal(t, []string{"create " + dependentIndependentID}, callsFor(calls, dependentIndependentID))

	saved := h.loadSaved(t)

	require.Equal(t, types.StatusFailed, resourceStatus(t, saved, dependentFirstID))
	require.Equal(t, types.StatusCreated, resourceStatus(t, saved, dependentIndependentID))

	independent := findResource[structs.Network](t, saved, dependentIndependentID)
	require.Equal(t, "id-independent", independent.ProviderID)

	_, err = saved.FindResource(dependentSecondID)
	require.Error(t, err)

	_, err = saved.FindResource(dependentThirdID)
	require.Error(t, err)
}

func TestFailedApplySavesProgress(t *testing.T) {
	h := setupLifecycle(t)
	h.plugin.SetCreateError(dependentSecondID, fmt.Errorf("container runtime unavailable"))

	st, err := h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	require.NotNil(t, st)
	require.Contains(t, err.Error(), "create failed for "+dependentSecondID)

	saved := h.loadSaved(t)

	require.Equal(t, types.StatusCreated, resourceStatus(t, saved, dependentFirstID))
	require.Equal(t, types.StatusFailed, resourceStatus(t, saved, dependentSecondID))

	first := findResource[structs.Network](t, saved, dependentFirstID)
	require.Equal(t, "id-first", first.ProviderID)
}

func TestNextApplyAfterFailureDoesNotRecreateSucceededResource(t *testing.T) {
	h := setupLifecycle(t)
	h.plugin.SetCreateError(dependentSecondID, fmt.Errorf("container runtime unavailable"))

	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	h.applyAndSave(t, lifecycleDependentConfig)

	calls := h.plugin.GetCalls()
	require.NotContains(t, calls, "create "+dependentFirstID)
	require.Equal(t, []string{
		"read " + dependentFirstID,
		"changed " + dependentFirstID,
	}, callsFor(calls, dependentFirstID))
}

func TestReadFailureIsSavedAsFailed(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetReadError(lifecycleNetworkID, fmt.Errorf("network API unavailable"))

	h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)

	saved := h.loadSaved(t)
	require.Equal(t, types.StatusFailed, networkStatus(t, saved))
}

func TestUnreachedPreviousResourceIsSavedUnchanged(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleDependentConfig)

	afterFirstApply := h.loadSaved(t)
	secondBefore := findResource[structs.Container](t, afterFirstApply, dependentSecondID)
	thirdBefore := findResource[structs.Container](t, afterFirstApply, dependentThirdID)
	require.Equal(t, types.StatusCreated, secondBefore.Meta.Status)
	require.Equal(t, types.StatusCreated, thirdBefore.Meta.Status)

	h.plugin.ResetCalls()
	h.plugin.SetReadError(dependentFirstID, fmt.Errorf("network API unavailable"))

	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)

	// second and third depend on first, so the walk never reached them
	calls := h.plugin.GetCalls()
	require.Empty(t, callsFor(calls, dependentSecondID))
	require.Empty(t, callsFor(calls, dependentThirdID))

	saved := h.loadSaved(t)

	secondAfter := findResource[structs.Container](t, saved, dependentSecondID)
	thirdAfter := findResource[structs.Container](t, saved, dependentThirdID)

	require.Equal(t, secondBefore, secondAfter)
	require.Equal(t, thirdBefore, thirdAfter)
	require.Equal(t, types.StatusCreated, secondAfter.Meta.Status)
	require.Equal(t, types.StatusCreated, thirdAfter.Meta.Status)
	// the addresses were assigned by the container Create in the first apply
	require.Equal(t, []structs.NetworkAttachment{{Name: "first", AssignedAddress: "assigned-first"}}, secondAfter.Networks)
	require.Equal(t, []structs.NetworkAttachment{{Name: "second", AssignedAddress: "assigned-second"}}, thirdAfter.Networks)
}

// requireOnlyAgreedStatuses reads the raw state file and requires every
// resource entry to carry one of the agreed statuses. Builtin types (variable,
// output) have no provider and are saved without a status.
func requireOnlyAgreedStatuses(t *testing.T, h *lifecycleHarness) {
	t.Helper()

	data, err := os.ReadFile(h.statePath)
	require.NoError(t, err)

	entries := []map[string]any{}
	err = json.Unmarshal(data, &entries)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	agreed := []string{
		types.StatusCreated,
		types.StatusUpdated,
		types.StatusFailed,
		types.StatusDestroyed,
		types.StatusDestroyFailed,
	}

	for _, entry := range entries {
		meta, ok := entry["meta"].(map[string]any)
		require.True(t, ok, "state entry has no meta: %v", entry)

		id := meta["id"]
		status, hasStatus := meta["status"]

		switch meta["type"] {
		case "variable", "output":
			require.False(t, hasStatus, "builtin %s should have no status, has %v", id, status)
		default:
			require.True(t, hasStatus, "resource %s has no status", id)
			require.Contains(t, agreed, status, "resource %s has status %v", id, status)
		}
	}
}

func TestOnlyAgreedStatusesAreSaved(t *testing.T) {
	h := setupLifecycle(t)

	// apply 1: first and independent succeed, second fails, third is never reached
	h.plugin.SetCreateError(dependentSecondID, fmt.Errorf("container runtime unavailable"))
	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	requireOnlyAgreedStatuses(t, h)

	// apply 2: everything succeeds, second is created again, third is created
	h.plugin.ClearErrors()
	h.applyAndSave(t, lifecycleDependentConfig)
	requireOnlyAgreedStatuses(t, h)

	// apply 3: reading independent fails
	h.plugin.SetReadError(dependentIndependentID, fmt.Errorf("network API unavailable"))
	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	requireOnlyAgreedStatuses(t, h)

	// apply 4: first is updated, independent is created again
	h.plugin.ClearErrors()
	h.plugin.SetChangedResult(dependentFirstID, true)
	h.applyAndSave(t, lifecycleDependentConfig)
	requireOnlyAgreedStatuses(t, h)

	saved := h.loadSaved(t)
	require.Equal(t, types.StatusUpdated, resourceStatus(t, saved, dependentFirstID))
}

// failNetworkCreate runs an apply of the single network config in which the
// network's Create fails, so the saved state holds the network as failed. The
// plugin's errors and calls are cleared afterwards.
func failNetworkCreate(t *testing.T, h *lifecycleHarness) {
	t.Helper()

	h.plugin.SetCreateError(lifecycleNetworkID, fmt.Errorf("network API unavailable"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)
	require.Contains(t, err.Error(), "create failed for "+lifecycleNetworkID)
	require.Equal(t, types.StatusFailed, networkStatus(t, h.loadSaved(t)))

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()
}

func TestFailedResourceIsDestroyedThenCreated(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{
		"destroy " + lifecycleNetworkID,
		"create " + lifecycleNetworkID,
	}, callsFor(h.plugin.GetCalls(), lifecycleNetworkID))
	require.Equal(t, types.StatusCreated, networkStatus(t, st))

	saved := h.loadSaved(t)
	require.Equal(t, types.StatusCreated, networkStatus(t, saved))
}

func TestFailedRebuildWithFailingDestroyIsSavedDestroyFailed(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)
	requireOnlyAgreedStatuses(t, h)

	h.plugin.SetDestroyError(lifecycleNetworkID, fmt.Errorf("network is still in use"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)
	require.Contains(t, err.Error(), "destroy failed for "+lifecycleNetworkID)

	// the destroy failed, so no create was attempted
	require.Equal(t, []string{
		"destroy " + lifecycleNetworkID,
	}, callsFor(h.plugin.GetCalls(), lifecycleNetworkID))

	saved := h.loadSaved(t)
	require.Equal(t, types.StatusDestroyFailed, networkStatus(t, saved))

	network := findResource[structs.Network](t, saved, lifecycleNetworkID)
	require.Equal(t, "10.0.0.0/16", network.Subnet)
	require.Equal(t, "one", network.Meta.Name)

	requireOnlyAgreedStatuses(t, h)
}

func TestDestroyFailedResourceIsRetriedOnNextApply(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	// apply 2: the rebuild's destroy fails, the network is saved destroy_failed
	h.plugin.SetDestroyError(lifecycleNetworkID, fmt.Errorf("network is still in use"))
	h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)
	require.Equal(t, types.StatusDestroyFailed, networkStatus(t, h.loadSaved(t)))

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	// apply 3: the destroy is retried, then the network is created
	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{
		"destroy " + lifecycleNetworkID,
		"create " + lifecycleNetworkID,
	}, callsFor(h.plugin.GetCalls(), lifecycleNetworkID))
	require.Equal(t, types.StatusCreated, networkStatus(t, st))

	saved := h.loadSaved(t)
	require.Equal(t, types.StatusCreated, networkStatus(t, saved))
	requireOnlyAgreedStatuses(t, h)
}

func TestRebuildDestroyReceivesSavedCopy(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Apply(lifecycleOriginalConfig)
	require.NoError(t, err)

	destroyStart := requireEvent(t, collector.all(), "destroy", "start", lifecycleNetworkID)

	sent := map[string]any{}
	err = json.Unmarshal(destroyStart.Data, &sent)
	require.NoError(t, err)

	// only the saved copy carries a status, the configured copy has none yet
	meta, ok := sent["meta"].(map[string]any)
	require.True(t, ok, "destroy data has no meta: %s", string(destroyStart.Data))
	require.Equal(t, "failed", meta["status"])
	require.Equal(t, "10.0.0.0/16", sent["subnet"])
}

func TestRebuildEventsUseDestroyThenCreate(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Apply(lifecycleOriginalConfig)
	require.NoError(t, err)

	require.Equal(t, []string{
		"destroy start",
		"destroy success",
		"create start",
		"create success",
	}, eventsFor(collector.all(), lifecycleNetworkID))
}

func TestRebuildEventsReportDestroyError(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)
	h.plugin.SetDestroyError(lifecycleNetworkID, fmt.Errorf("network is still in use"))

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Apply(lifecycleOriginalConfig)
	require.Error(t, err)

	events := collector.all()
	errorEvent := requireEvent(t, events, "destroy", "error", lifecycleNetworkID)
	require.ErrorContains(t, errorEvent.Error, "network is still in use")

	require.Equal(t, []string{
		"destroy start",
		"destroy error",
	}, eventsFor(events, lifecycleNetworkID))
}

func TestDependentsOfFailedRebuildAreNotProcessed(t *testing.T) {
	h := setupLifecycle(t)

	// apply 1: first fails, so its dependents are never reached
	h.plugin.SetCreateError(dependentFirstID, fmt.Errorf("network API unavailable"))
	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	require.Equal(t, types.StatusFailed, resourceStatus(t, h.loadSaved(t), dependentFirstID))

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	// apply 2: the rebuild of first fails at its destroy
	h.plugin.SetDestroyError(dependentFirstID, fmt.Errorf("network is still in use"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	require.Contains(t, err.Error(), "destroy failed for "+dependentFirstID)

	calls := h.plugin.GetCalls()
	require.Equal(t, []string{"destroy " + dependentFirstID}, callsFor(calls, dependentFirstID))
	require.Empty(t, callsFor(calls, dependentSecondID))
	require.Empty(t, callsFor(calls, dependentThirdID))
	require.Equal(t, []string{
		"read " + dependentIndependentID,
		"changed " + dependentIndependentID,
	}, callsFor(calls, dependentIndependentID))

	saved := h.loadSaved(t)
	require.Equal(t, types.StatusDestroyFailed, resourceStatus(t, saved, dependentFirstID))
	requireOnlyAgreedStatuses(t, h)
}

func TestComputedValuesSurviveUnchangedApply(t *testing.T) {
	h := setupLifecycle(t)

	// apply 1: the network's Create sets its provider id, the container's
	// network block is named after it
	h.applyAndSave(t, lifecycleComputedRefConfig)
	h.plugin.ResetCalls()

	// apply 2: the test plugin's Read adds nothing, so the provider id survives
	// only because the parser carries it over from the saved copy
	st := h.applyAndSave(t, lifecycleComputedRefConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Equal(t, []string{
		"read " + lifecycleNetworkID,
		"changed " + lifecycleNetworkID,
	}, callsFor(h.plugin.GetCalls(), lifecycleNetworkID))
	require.Equal(t, []string{
		"read " + computedRefUserID,
		"changed " + computedRefUserID,
	}, callsFor(h.plugin.GetCalls(), computedRefUserID))

	saved := h.loadSaved(t)
	network := findResource[structs.Network](t, saved, lifecycleNetworkID)
	require.Equal(t, "id-one", network.ProviderID)
	require.Equal(t, types.StatusCreated, network.Meta.Status)

	// the dependent resolved its reference to the carried provider id
	user := findResource[structs.Container](t, st, computedRefUserID)
	require.Equal(t, []structs.NetworkAttachment{
		{Name: "id-one", AssignedAddress: "assigned-id-one"},
	}, user.Networks)
	require.Equal(t, types.StatusCreated, user.Meta.Status)
}

func TestNestedComputedValueSurvivesUnchangedApply(t *testing.T) {
	h := setupLifecycle(t)

	// apply 1: the container Create assigns an address to each network block,
	// consumer names its network block after web's first assigned address
	h.applyAndSave(t, lifecycleNestedConfig)
	h.plugin.ResetCalls()

	st := h.applyAndSave(t, lifecycleNestedConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())

	saved := h.loadSaved(t)
	web := findResource[structs.Container](t, saved, nestedWebID)
	require.Equal(t, []structs.NetworkAttachment{
		{ID: 1, Name: "n1", AssignedAddress: "assigned-n1"},
		{ID: 2, Name: "n2", AssignedAddress: "assigned-n2"},
	}, web.Networks)

	// the dependent resolved its reference to the carried nested address
	consumer := findResource[structs.Container](t, st, nestedConsumerID)
	require.Equal(t, []structs.NetworkAttachment{
		{Name: "assigned-n1", AssignedAddress: "assigned-assigned-n1"},
	}, consumer.Networks)
}

func TestNestedComputedValueFollowsKeyWhenBlocksReordered(t *testing.T) {
	h := setupLifecycle(t)

	// apply 1: n1 (id 1) is first, n2 (id 2) is second
	h.applyAndSave(t, lifecycleReorderedOriginal)
	h.plugin.ResetCalls()

	// apply 2: the same blocks, swapped in the config. Default change detection
	// compares the lists in order, so the swap may legitimately be seen as a
	// change and trigger an update; this test only asserts on the carry-over,
	// which is visible in the configured copy passed to Read.
	h.applyAndSave(t, lifecycleReorderedSwapped)

	readCalls := h.plugin.GetReadCalls()
	require.Len(t, readCalls, 1)
	require.Equal(t, nestedWebID, readCalls[0].ID)

	configured := structs.Container{}
	err := json.Unmarshal(readCalls[0].New, &configured)
	require.NoError(t, err)

	// each address followed its block's id, not its position
	require.Equal(t, []structs.NetworkAttachment{
		{ID: 2, Name: "n2", AssignedAddress: "assigned-n2"},
		{ID: 1, Name: "n1", AssignedAddress: "assigned-n1"},
	}, configured.Networks)
}

func TestProviderChangingConfiguredValueWarns(t *testing.T) {
	h := setupLifecycle(t)
	log := &recordingLogger{}
	h.log = log

	h.plugin.SetMutateConfigured(lifecycleNetworkID, "10.9.0.0/16")

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	// the warning never fails the apply, the provider's value is kept
	network := findResource[structs.Network](t, st, lifecycleNetworkID)
	require.Equal(t, "10.9.0.0/16", network.Subnet)
	require.Equal(t, types.StatusCreated, network.Meta.Status)

	require.Equal(t, [][]any{
		{"resource", lifecycleNetworkID, "field", "subnet"},
	}, log.warningsWithMessage(changedConfiguredValueWarning))
}

func TestProviderSettingComputedFieldDoesNotWarn(t *testing.T) {
	h := setupLifecycle(t)
	log := &recordingLogger{}
	h.log = log

	// apply 1: Create sets the computed provider id
	h.applyAndSave(t, lifecycleOriginalConfig)
	require.Empty(t, log.warningsWithMessage(changedConfiguredValueWarning))

	// apply 2: the provider id is carried over and read back
	h.applyAndSave(t, lifecycleOriginalConfig)
	require.Empty(t, log.warningsWithMessage(changedConfiguredValueWarning))

	network := findResource[structs.Network](t, h.loadSaved(t), lifecycleNetworkID)
	require.Equal(t, "id-one", network.ProviderID)
}

func TestReferenceSetFieldDoesNotWarn(t *testing.T) {
	h := setupLifecycle(t)
	log := &recordingLogger{}
	h.log = log

	// b's subnet is set by reference, so the provider changing it is not a
	// change to a value the user configured
	h.plugin.SetMutateConfigured(referenceReferenceID, "other")

	st := h.applyAndSave(t, lifecycleReferenceConfig)

	b := findResource[structs.Network](t, st, referenceReferenceID)
	require.Equal(t, "other", b.Subnet)
	require.Empty(t, log.warningsWithMessage(changedConfiguredValueWarning))
}
