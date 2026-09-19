package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser/mocks"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	lifecycleDisabledConfig = "../test_fixtures/config/lifecycle/disabled/disabled.xcl"

	disabledOnID  = "resource.network.on"
	disabledOffID = "resource.network.off"

	dependentVariableID = "variable.independent_subnet"
	dependentOutputID   = "output.first_name"
)

// dependentProviderIDs are the resources of the dependent fixture a provider
// creates and destroys, the variable and output are builtin types.
var dependentProviderIDs = []string{
	dependentFirstID,
	dependentSecondID,
	dependentThirdID,
	dependentIndependentID,
}

// dependentParents is, for every resource of the dependent fixture, the
// resources it depends on. None of them may be destroyed while it remains.
var dependentParents = map[string][]string{
	dependentFirstID:       {},
	dependentSecondID:      {dependentFirstID},
	dependentThirdID:       {dependentSecondID},
	dependentIndependentID: {dependentVariableID},
	dependentOutputID:      {dependentFirstID},
	dependentVariableID:    {},
}

// destroyAll loads the state the harness store last saved and destroys it with
// a fresh Parser, just as a separate destroy run would. onEvent may be nil.
func destroyAll(t *testing.T, h *lifecycleHarness, onEvent func(ParserEvent)) (*state.State, error) {
	t.Helper()

	p := h.newParser(t, onEvent)

	loaded, err := h.store.Load()
	require.NoError(t, err)

	return p.Destroy(loaded)
}

// destroyCalls returns the IDs the provider was asked to destroy, in order.
func destroyCalls(p *TestPlugin) []string {
	ids := []string{}
	for _, call := range p.GetCalls() {
		id, found := strings.CutPrefix(call, "destroy ")
		if found {
			ids = append(ids, id)
		}
	}

	return ids
}

// stateIDs returns the sorted IDs of every resource in st.
func stateIDs(t *testing.T, st *state.State) []string {
	t.Helper()

	ids := []string{}
	for _, r := range st.GetResources() {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)

	return ids
}

// snapshotIDs returns the sorted IDs of every resource in a serialized state.
func snapshotIDs(t *testing.T, snapshot []byte) []string {
	t.Helper()

	resources := []struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}{}

	err := json.Unmarshal(snapshot, &resources)
	require.NoError(t, err)

	ids := []string{}
	for _, r := range resources {
		ids = append(ids, r.Meta.ID)
	}

	sort.Strings(ids)

	return ids
}

// sorted returns a sorted copy of ids.
func sorted(ids ...string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)

	return out
}

// recordingStore is a state store that saves through another store and keeps
// a copy of every state it was asked to save.
type recordingStore struct {
	store state.StateStore

	mu        sync.Mutex
	snapshots [][]byte
}

func (s *recordingStore) Load() (*state.State, error) {
	return s.store.Load()
}

func (s *recordingStore) Save(st *state.State) error {
	snapshot, err := st.Bytes()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.snapshots = append(s.snapshots, snapshot)
	s.mu.Unlock()

	return s.store.Save(st)
}

func (s *recordingStore) Exists() bool {
	return s.store.Exists()
}

func (s *recordingStore) Clear() error {
	return s.store.Clear()
}

func (s *recordingStore) all() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([][]byte{}, s.snapshots...)
}

// destroyRecordingSaves destroys the saved state of the harness with a parser
// whose store records every save, and returns the recorded snapshots.
func destroyRecordingSaves(t *testing.T, h *lifecycleHarness) [][]byte {
	t.Helper()

	recording := &recordingStore{store: h.store}

	options := testOptions(t)
	options.Logger = logger.NewTestLogger(t)
	options.PluginRegistry = h.registry
	options.StateStore = recording

	p := NewParser(options)

	loaded, err := h.store.Load()
	require.NoError(t, err)

	remaining, err := p.Destroy(loaded)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	return recording.all()
}

func TestDestroyCallsProvidersChildrenFirst(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	requireBefore(t, dependentThirdID, dependentSecondID, calls)
	requireBefore(t, dependentSecondID, dependentFirstID, calls)
	require.Contains(t, calls, dependentIndependentID)
}

func TestDestroyRemovesEveryResourceFromSavedState(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	require.Equal(t, 6, h.loadSaved(t).ResourceCount())

	remaining, err := destroyAll(t, h, nil)
	require.NoError(t, err)
	require.NotNil(t, remaining)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Equal(t, 0, h.loadSaved(t).ResourceCount())
}

func TestDestroyCallsEachProviderResourceExactlyOnce(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	require.Len(t, calls, 4)
	require.ElementsMatch(t, dependentProviderIDs, calls)
}

func TestDestroyFailureLeavesParentsAndDestroysUnrelated(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	remaining, err := destroyAll(t, h, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), dependentSecondID)
	require.Contains(t, err.Error(), "boom")

	calls := destroyCalls(h.plugin)
	require.Contains(t, calls, dependentThirdID)
	require.Contains(t, calls, dependentIndependentID)
	require.Contains(t, calls, dependentSecondID)
	require.NotContains(t, calls, dependentFirstID)

	require.Equal(t, sorted(dependentSecondID, dependentFirstID), stateIDs(t, remaining))

	saved := h.loadSaved(t)
	require.Equal(t, sorted(dependentSecondID, dependentFirstID), stateIDs(t, saved))
	require.Equal(t, types.StatusDestroyFailed, resourceStatus(t, saved, dependentSecondID))
	require.Equal(t, types.StatusCreated, resourceStatus(t, saved, dependentFirstID))
}

func TestDestroyRetriesFailedResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	_, err := destroyAll(t, h, nil)
	require.Error(t, err)

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	remaining, err := destroyAll(t, h, nil)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Equal(t, []string{dependentSecondID, dependentFirstID}, destroyCalls(h.plugin))
	require.Equal(t, 0, h.loadSaved(t).ResourceCount())
}

func TestDestroySavesStateAfterEachResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)

	// one save for each of the six resources
	require.Len(t, snapshots, 6)

	previous := sorted(
		dependentFirstID,
		dependentSecondID,
		dependentThirdID,
		dependentIndependentID,
		dependentOutputID,
		dependentVariableID,
	)

	for i, snapshot := range snapshots {
		current := snapshotIDs(t, snapshot)

		// each save holds exactly one resource fewer than the one before
		require.Len(t, current, len(previous)-1, "snapshot %d: %v", i, current)
		for _, id := range current {
			require.Contains(t, previous, id, "snapshot %d holds %s, destroyed earlier", i, id)
		}

		// a resource that remains still has every resource it depends on
		for _, id := range current {
			for _, parent := range dependentParents[id] {
				require.Contains(t, current, parent, "snapshot %d holds %s but not its parent %s", i, id, parent)
			}
		}

		previous = current
	}

	require.Empty(t, previous)
}

func TestDestroyFirstSaveRemovesAResourceNothingDependsOn(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)
	require.NotEmpty(t, snapshots)

	all := sorted(
		dependentFirstID,
		dependentSecondID,
		dependentThirdID,
		dependentIndependentID,
		dependentOutputID,
		dependentVariableID,
	)

	first := snapshotIDs(t, snapshots[0])

	removed := []string{}
	for _, id := range all {
		if !contains(first, id) {
			removed = append(removed, id)
		}
	}

	require.Len(t, removed, 1)
	require.Contains(t, []string{dependentThirdID, dependentIndependentID, dependentOutputID}, removed[0])
}

func TestInterruptedDestroyResumesFromSavedState(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)
	require.Len(t, snapshots, 6)

	// restart the destroy from every point the first run saved, as if it had
	// been interrupted straight after that save
	for i, snapshot := range snapshots {
		err := os.WriteFile(h.statePath, snapshot, 0644)
		require.NoError(t, err)

		remainingProviderIDs := []string{}
		for _, id := range snapshotIDs(t, snapshot) {
			if contains(dependentProviderIDs, id) {
				remainingProviderIDs = append(remainingProviderIDs, id)
			}
		}

		h.plugin.ResetCalls()

		remaining, err := destroyAll(t, h, nil)
		require.NoError(t, err, "resuming from snapshot %d", i)
		require.Equal(t, 0, remaining.ResourceCount())

		require.ElementsMatch(t, remainingProviderIDs, destroyCalls(h.plugin), "resuming from snapshot %d", i)
		require.Equal(t, 0, h.loadSaved(t).ResourceCount())
	}
}

func TestInterruptedDestroyAfterChildDestroyedNeverDestroysItAgain(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)

	// find the first save that no longer holds third, the rest of the chain is
	// still there
	resumeFrom := -1
	for i, snapshot := range snapshots {
		if !contains(snapshotIDs(t, snapshot), dependentThirdID) {
			resumeFrom = i
			break
		}
	}
	require.NotEqual(t, -1, resumeFrom)

	err := os.WriteFile(h.statePath, snapshots[resumeFrom], 0644)
	require.NoError(t, err)
	h.plugin.ResetCalls()

	_, err = destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	require.NotContains(t, calls, dependentThirdID)
	requireBefore(t, dependentSecondID, dependentFirstID, calls)
}

func TestDestroyNeverCallsProviderForBuiltinAndRegisteredBlocks(t *testing.T) {
	h := setupRegisteredTypes(t)
	h.applyAndSave(t, registeredBasicConfig)

	collector := &eventCollector{}

	options := testOptions(t)
	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.OnParserEvent = collector.collect
	// no expectations, any provider lookup fails the test
	options.ProviderResolver = mocks.NewMockProviderResolver(t)

	p := NewParser(options)

	loaded, err := h.store.Load()
	require.NoError(t, err)
	require.Equal(t, 7, loaded.ResourceCount())

	remaining, err := p.Destroy(loaded)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.Equal(t, 0, saved.ResourceCount())

	events := collector.all()
	for _, id := range []string{
		registeredVariableID,
		registeredDatabaseID,
		"module.shared",
		registeredModuleDatabaseID,
		"module.shared.output.location",
		registeredAppID,
		registeredConsumerID,
	} {
		require.Equal(t, []string{"destroy success"}, eventsFor(events, id), id)
	}
}

func TestDestroyNeverCallsProviderForDisabledRegisteredBlock(t *testing.T) {
	h := setupRegisteredTypes(t)
	h.applyAndSave(t, registeredDisabledConfig)

	collector := &eventCollector{}

	options := testOptions(t)
	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.OnParserEvent = collector.collect
	// no expectations, any provider lookup fails the test
	options.ProviderResolver = mocks.NewMockProviderResolver(t)

	p := NewParser(options)

	loaded, err := h.store.Load()
	require.NoError(t, err)
	require.Equal(t, []string{registeredDisabledID}, stateIDs(t, loaded))

	remaining, err := p.Destroy(loaded)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.Equal(t, 0, saved.ResourceCount())

	require.Equal(t, []string{"destroy success"}, eventsFor(collector.all(), registeredDisabledID))
}

func TestDestroyNeverCallsProviderForDisabledProviderBlock(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDisabledConfig)
	require.Equal(t, sorted(disabledOnID, disabledOffID), stateIDs(t, h.loadSaved(t)))
	h.plugin.ResetCalls()

	collector := &eventCollector{}

	remaining, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Equal(t, []string{disabledOnID}, destroyCalls(h.plugin))
	require.Equal(t, []string{"destroy success"}, eventsFor(collector.all(), disabledOffID))
	require.Equal(t, 0, h.loadSaved(t).ResourceCount())
}

func TestDestroyOrphansNothingWhenAResourceFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	created := h.plugin.GetCreatedResources()
	require.ElementsMatch(t, dependentProviderIDs, created)

	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	_, err := destroyAll(t, h, nil)
	require.Error(t, err)

	destroyedByProvider := []string{}
	for _, id := range destroyCalls(h.plugin) {
		if id != dependentSecondID {
			destroyedByProvider = append(destroyedByProvider, id)
		}
	}

	saved := stateIDs(t, h.loadSaved(t))

	// every resource a provider created is either gone or still saved
	for _, id := range created {
		if contains(destroyedByProvider, id) {
			require.NotContains(t, saved, id)
			continue
		}

		require.Contains(t, saved, id, "%s was created but is neither destroyed nor saved", id)
	}
}

func TestDestroyNeverDestroysParentOfFailedChild(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	_, err := destroyAll(t, h, nil)
	require.Error(t, err)

	calls := destroyCalls(h.plugin)
	saved := h.loadSaved(t)

	failed := 0
	for _, r := range saved.GetResources() {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		if meta.Status != types.StatusDestroyFailed {
			continue
		}

		failed++
		for _, parent := range meta.Parents {
			require.NotContains(t, calls, parent, "%s was destroyed after its child %s failed", parent, meta.ID)
		}
	}

	require.Equal(t, 1, failed)
}

func TestDestroyWithEmptyStateCallsNoProvider(t *testing.T) {
	h := setupLifecycle(t)
	p := h.newParser(t, nil)

	remaining, err := p.Destroy(state.NewState())
	require.NoError(t, err)
	require.NotNil(t, remaining)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Empty(t, h.plugin.GetCalls())
}

func TestDestroyWithNilStateCallsNoProvider(t *testing.T) {
	h := setupLifecycle(t)
	p := h.newParser(t, nil)

	remaining, err := p.Destroy(nil)
	require.NoError(t, err)
	require.NotNil(t, remaining)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Empty(t, h.plugin.GetCalls())
}

func TestDestroyFiresStartThenSuccessForProviderResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)

	require.Equal(t, []string{"destroy start", "destroy success"}, eventsFor(collector.all(), dependentFirstID))
}

func TestDestroyFiresStartThenErrorForFailingResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.Error(t, err)

	events := collector.all()
	require.Equal(t, []string{"destroy start", "destroy error"}, eventsFor(events, dependentSecondID))
	require.Empty(t, eventsFor(events, dependentFirstID))
}

func TestDestroyFiresOnlySuccessForVariable(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)

	events := collector.all()
	require.Equal(t, []string{"destroy success"}, eventsFor(events, dependentVariableID))
	require.Equal(t, []string{"destroy success"}, eventsFor(events, dependentOutputID))
}

// contains reports whether ids holds id.
func contains(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}

	return false
}
