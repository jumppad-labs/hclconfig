package parser

import (
	"fmt"
	"sync"

	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// outcome is what happened to a resource that the walk reached
type outcome struct {
	// failed is true when a provider call for the resource failed
	failed bool

	// saved is the copy of the resource to persist
	saved any
}

// applyProgress records the outcome of every resource the walk reaches. The
// walker visits resources in parallel and exposes no per-resource results, so
// the walk callback records them here. It is safe for concurrent use.
type applyProgress struct {
	mu       sync.Mutex
	outcomes map[string]outcome
}

func newApplyProgress() *applyProgress {
	return &applyProgress{outcomes: map[string]outcome{}}
}

// record stores the outcome for the resource with the given ID
func (p *applyProgress) record(id string, o outcome) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.outcomes[id] = o
}

// buildState builds the state to save after a failed walk:
//
//   - reached resources are saved as they are now, including failed ones
//   - resources that were not reached keep their entry from the previous state
//   - new resources that were not reached are left out
func (p *applyProgress) buildState(current, previous *state.State) (*state.State, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	built := state.NewState()

	for _, r := range current.GetResources() {
		meta, err := types.GetMeta(r)
		if err != nil {
			return nil, err
		}

		if o, reached := p.outcomes[meta.ID]; reached {
			if err := built.AppendResource(o.saved); err != nil {
				return nil, fmt.Errorf("unable to save progress for %s: %w", meta.ID, err)
			}

			continue
		}

		previousResource, err := previous.FindResource(meta.ID)
		if err != nil {
			// a new resource that was never reached
			continue
		}

		if err := built.AppendResource(previousResource); err != nil {
			return nil, fmt.Errorf("unable to save previous entry for %s: %w", meta.ID, err)
		}
	}

	return built, nil
}
