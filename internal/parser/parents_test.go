package parser

import (
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	lifecycleModuleReferenceConfig = "../test_fixtures/config/lifecycle/module_reference/main.xcl"

	moduleReferenceModuleID    = "module.networks"
	moduleReferenceOneID       = "module.networks.resource.network.one"
	moduleReferenceTwoID       = "module.networks.resource.network.two"
	moduleReferenceConsumerID  = "resource.network.consumer"
	moduleReferenceAloneID     = "resource.network.alone"
	dependentIndependentVarID  = "variable.independent_subnet"
	dependentFirstNameOutputID = "output.first_name"
	registeredSharedModuleID   = "module.shared"
	registeredModuleOutputID   = "module.shared.output.location"
)

// savedIDs returns the ID of every resource in st, in state order.
func savedIDs(t *testing.T, st *state.State) []string {
	t.Helper()

	ids := []string{}
	for _, resource := range st.GetResources() {
		meta, err := types.GetMeta(resource)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	return ids
}

// After an apply every saved resource lists the resources it depends on as its
// parents, and a resource with no dependencies lists none.
func TestApplyRecordsParentsOfEachResource(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleDependentConfig)
	saved := h.loadSaved(t)

	require.Empty(t, requireMeta(t, saved, dependentFirstID).Parents)
	require.Equal(t, []string{dependentFirstID}, requireMeta(t, saved, dependentSecondID).Parents)
	require.Equal(t, []string{dependentSecondID}, requireMeta(t, saved, dependentThirdID).Parents)
	require.Equal(t, []string{dependentIndependentVarID}, requireMeta(t, saved, dependentIndependentID).Parents)
	require.Equal(t, []string{dependentFirstID}, requireMeta(t, saved, dependentFirstNameOutputID).Parents)
	require.Empty(t, requireMeta(t, saved, dependentIndependentVarID).Parents)
}

// A resource inside a module lists that module as a parent, alongside anything
// else it depends on.
func TestApplyRecordsModuleAsParentOfItsResources(t *testing.T) {
	h := setupRegisteredTypes(t)

	h.applyAndSave(t, registeredBasicConfig)
	saved, err := h.store.Load()
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{registeredSharedModuleID},
		requireMeta(t, saved, registeredModuleDatabaseID).Parents,
	)
	require.Equal(
		t,
		[]string{registeredSharedModuleID, registeredModuleDatabaseID},
		requireMeta(t, saved, registeredModuleOutputID).Parents,
	)
}

// A depends_on naming a whole module lists every resource in that module as a
// parent, while a resource that depends on nothing lists none.
func TestApplyRecordsEveryModuleResourceForModuleReference(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleModuleReferenceConfig)
	saved := h.loadSaved(t)

	require.Equal(
		t,
		[]string{moduleReferenceOneID, moduleReferenceTwoID},
		requireMeta(t, saved, moduleReferenceConsumerID).Parents,
	)
	require.Equal(t, []string{moduleReferenceModuleID}, requireMeta(t, saved, moduleReferenceOneID).Parents)
	require.Equal(t, []string{moduleReferenceModuleID}, requireMeta(t, saved, moduleReferenceTwoID).Parents)
	require.Empty(t, requireMeta(t, saved, moduleReferenceAloneID).Parents)
}

// Re-applying an unchanged configuration makes no provider destroy call and
// saves the same resources, with the same parents, as the first apply.
func TestReapplyWithoutChangesDestroysNothingAndKeepsParents(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleDependentConfig)
	first := h.loadSaved(t)

	h.plugin.ResetCalls()

	h.applyAndSave(t, lifecycleDependentConfig)
	second := h.loadSaved(t)

	for _, call := range h.plugin.GetCalls() {
		require.False(t, strings.HasPrefix(call, "destroy "), "unexpected provider call %q", call)
	}
	require.Empty(t, h.plugin.GetDestroyedResources())

	expectedIDs := []string{
		dependentIndependentVarID,
		dependentFirstID,
		dependentSecondID,
		dependentThirdID,
		dependentIndependentID,
		dependentFirstNameOutputID,
	}
	require.ElementsMatch(t, expectedIDs, savedIDs(t, first))
	require.ElementsMatch(t, expectedIDs, savedIDs(t, second))

	require.Equal(t, requireMeta(t, first, dependentFirstID).Parents, requireMeta(t, second, dependentFirstID).Parents)
	require.Equal(t, requireMeta(t, first, dependentSecondID).Parents, requireMeta(t, second, dependentSecondID).Parents)
	require.Equal(t, requireMeta(t, first, dependentThirdID).Parents, requireMeta(t, second, dependentThirdID).Parents)
	require.Equal(t, requireMeta(t, first, dependentIndependentID).Parents, requireMeta(t, second, dependentIndependentID).Parents)
	require.Equal(t, requireMeta(t, first, dependentIndependentVarID).Parents, requireMeta(t, second, dependentIndependentVarID).Parents)
	require.Equal(t, requireMeta(t, first, dependentFirstNameOutputID).Parents, requireMeta(t, second, dependentFirstNameOutputID).Parents)

	require.Equal(t, []string{dependentSecondID}, requireMeta(t, second, dependentThirdID).Parents)
}
