package xcl

import (
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// Diff represents the changes that would be applied
// ToUpdate will initially include all resources that exist in both old and new state
// Later we can add deeper comparison to only include resources with actual changes
type Diff struct {
	ToCreate  []any // Resources that would be created
	ToUpdate  []any // Resources that would be updated
	ToDestroy []any // Resources that would be destroyed
}

// buildDiff compares two states and returns a Diff showing what changed
func buildDiff(newState, existingState *state.State) *Diff {
	diff := &Diff{
		ToCreate:  []any{},
		ToUpdate:  []any{},
		ToDestroy: []any{},
	}

	if existingState == nil || existingState.ResourceCount() == 0 {
		// First run - everything is a create
		diff.ToCreate = newState.GetResources()
		return diff
	}

	// Build maps by resource ID for comparison
	existingMap := make(map[string]any)
	for _, r := range existingState.GetResources() {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		existingMap[meta.ID] = r
	}

	newMap := make(map[string]any)
	for _, r := range newState.GetResources() {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		newMap[meta.ID] = r
	}

	// Find creates and updates
	for id, newRes := range newMap {
		if _, exists := existingMap[id]; exists {
			diff.ToUpdate = append(diff.ToUpdate, newRes)
		} else {
			diff.ToCreate = append(diff.ToCreate, newRes)
		}
	}

	// Find destroys
	for id, oldRes := range existingMap {
		if _, exists := newMap[id]; !exists {
			diff.ToDestroy = append(diff.ToDestroy, oldRes)
		}
	}

	return diff
}
