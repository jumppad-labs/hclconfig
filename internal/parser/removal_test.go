package parser

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"testing"

	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	removalBeforeConfig        = "../test_fixtures/config/lifecycle/removal/before/main.xcl"
	removalWithoutXConfig      = "../test_fixtures/config/lifecycle/removal/without_x/main.xcl"
	removalWithoutXWithZConfig = "../test_fixtures/config/lifecycle/removal/without_x_with_z/main.xcl"
	removalWithoutPQConfig     = "../test_fixtures/config/lifecycle/removal/without_pq/main.xcl"

	emptyConfigDir = "../test_fixtures/config/empty"

	removalXID = "resource.network.x"
	removalYID = "resource.network.y"
	removalZID = "resource.network.z"
	removalPID = "resource.network.p"
	removalQID = "resource.container.q"
)

// savedResourceWithoutLocation returns the resource with the given ID as it is
// held in st, decoded from its JSON, without the file, line and column its
// block was read from, those follow the configuration file being applied.
func savedResourceWithoutLocation(t *testing.T, st *state.State, resourceID string) map[string]any {
	t.Helper()

	resource, err := st.FindResource(resourceID)
	require.NoError(t, err)

	data, err := json.Marshal(resource)
	require.NoError(t, err)

	decoded := map[string]any{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	meta, ok := decoded["meta"].(map[string]any)
	require.True(t, ok)
	delete(meta, "file")
	delete(meta, "line")
	delete(meta, "column")

	return decoded
}

func TestApplyDestroysRemovedResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)

	yBefore := savedResourceWithoutLocation(t, h.loadSaved(t), removalYID)
	h.plugin.ResetCalls()

	h.applyAndSave(t, removalWithoutXConfig)

	calls := h.plugin.GetCalls()
	require.Contains(t, calls, "destroy "+removalXID)
	require.Equal(t, []string{removalXID}, destroyCalls(h.plugin))

	// y is untouched, it is neither created nor updated again
	require.NotContains(t, calls, "create "+removalYID)
	require.NotContains(t, calls, "update "+removalYID)

	saved := h.loadSaved(t)
	require.Equal(t, sorted(removalYID, removalPID, removalQID), stateIDs(t, saved))
	require.Equal(t, yBefore, savedResourceWithoutLocation(t, saved, removalYID))
}

func TestApplyDestroysRemovedBeforeCreatingNew(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)
	h.plugin.ResetCalls()

	h.applyAndSave(t, removalWithoutXWithZConfig)

	calls := h.plugin.GetCalls()
	requireBefore(t, "destroy "+removalXID, "create "+removalZID, calls)

	saved := h.loadSaved(t)
	require.Equal(t, sorted(removalYID, removalPID, removalQID, removalZID), stateIDs(t, saved))
}

func TestApplyDestroysRemovedChildBeforeRemovedParent(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)
	h.plugin.ResetCalls()

	h.applyAndSave(t, removalWithoutPQConfig)

	calls := destroyCalls(h.plugin)
	require.Len(t, calls, 2)
	requireBefore(t, removalQID, removalPID, calls)

	saved := h.loadSaved(t)
	require.Equal(t, sorted(removalXID, removalYID), stateIDs(t, saved))
}

func TestApplyStopsWhenRemovalFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)
	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(removalXID, fmt.Errorf("boom"))

	_, err := h.applyAndSaveExpectingFailure(t, removalWithoutXWithZConfig)
	require.Contains(t, err.Error(), removalXID)

	calls := h.plugin.GetCalls()
	require.Contains(t, calls, "destroy "+removalXID)
	require.NotContains(t, calls, "create "+removalZID)

	saved := h.loadSaved(t)
	require.Equal(t, sorted(removalXID, removalYID, removalPID, removalQID), stateIDs(t, saved))
	require.Equal(t, types.StatusDestroyFailed, resourceStatus(t, saved, removalXID))
}

func TestApplyRetriesFailedRemovalBeforeCreating(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)
	h.plugin.SetDestroyError(removalXID, fmt.Errorf("boom"))

	_, err := h.applyAndSaveExpectingFailure(t, removalWithoutXWithZConfig)
	require.Error(t, err)

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	h.applyAndSave(t, removalWithoutXWithZConfig)

	calls := h.plugin.GetCalls()
	requireBefore(t, "destroy "+removalXID, "create "+removalZID, calls)

	saved := h.loadSaved(t)
	require.Equal(t, sorted(removalYID, removalPID, removalQID, removalZID), stateIDs(t, saved))
}

func TestApplyRejectsEmptyConfiguration(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)

	before, err := os.ReadFile(h.statePath)
	require.NoError(t, err)
	callCount := len(h.plugin.GetCalls())

	p := h.newParser(t, nil)

	st, err := p.Apply(emptyConfigDir)
	require.Error(t, err)
	require.True(t, stderrors.Is(err, ErrEmptyConfiguration), "unexpected error: %v", err)
	require.Nil(t, st)

	require.Len(t, h.plugin.GetCalls(), callCount)

	after, err := os.ReadFile(h.statePath)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func TestRemovalOrphansNothingWhenADestroyFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)

	created := h.plugin.GetCreatedResources()
	require.ElementsMatch(t, []string{removalXID, removalYID, removalPID, removalQID}, created)

	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(removalQID, fmt.Errorf("boom"))

	_, err := h.applyAndSaveExpectingFailure(t, removalWithoutPQConfig)
	require.Contains(t, err.Error(), removalQID)

	destroyedByProvider := []string{}
	for _, id := range destroyCalls(h.plugin) {
		if id != removalQID {
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

func TestRemovalNeverDestroysParentOfFailedChild(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, removalBeforeConfig)
	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(removalQID, fmt.Errorf("boom"))

	_, err := h.applyAndSaveExpectingFailure(t, removalWithoutPQConfig)
	require.Error(t, err)

	calls := destroyCalls(h.plugin)
	require.Equal(t, []string{removalQID}, calls)
	require.NotContains(t, calls, removalPID)

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
	require.Equal(t, types.StatusDestroyFailed, resourceStatus(t, saved, removalQID))
	require.Contains(t, stateIDs(t, saved), removalPID)
}
