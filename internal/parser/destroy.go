package parser

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/hashicorp/errwrap"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/dag"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// destroyer destroys a set of resources held in a working copy of the saved
// state. It walks them children first and, after each resource, updates the
// working state and saves it through the store: a destroyed resource is
// removed, a resource whose destroy failed is kept and marked destroy_failed.
// The saved state is therefore correct at every step, and an interrupted
// destroy resumes from it.
type destroyer struct {
	// working is the state being changed, it starts as the saved state and
	// ends holding what survives the destroy
	working *state.State

	// store persists working after every resource, it may be nil in which
	// case nothing is persisted
	store state.StateStore

	resolver ProviderResolver
	types    TypeRegistry
	options  *ParserOptions

	// mu guards working and the store, the walk visits unrelated resources
	// concurrently
	mu sync.Mutex
}

// destroy walks targets, a subset of the working state, children first. Every
// resource that fails, and every resource whose children did not all go, stays
// in the working state. The returned error names every failed resource.
func (d *destroyer) destroy(targets []any) error {
	if len(targets) == 0 {
		return nil
	}

	graph, err := buildDestroyDAG(targets)
	if err != nil {
		return err
	}

	// Reduce the graph nodes to unique instances
	graph.TransitiveReduction()

	err = graph.Validate()
	if err != nil {
		return fmt.Errorf("unable to validate destroy dependency graph: %w", err)
	}

	// the graph runs parent to child like the create graph, walking it in
	// reverse visits every child before its parents and never visits a parent
	// once one of its children has failed
	w := dag.Walker{
		Callback: destroyWalkCallback(d),
		Reverse:  true,
	}

	log.SetOutput(io.Discard)

	w.Update(graph)
	diags := w.Wait()
	if !diags.HasErrors() {
		return nil
	}

	ce := errors.NewConfigError()
	for _, e := range diags.Err().(errwrap.Wrapper).WrappedErrors() {
		ce.AppendError(e)
	}

	return ce
}

// destroyed removes a destroyed resource from the working state and saves it
func (d *destroyer) destroyed(r any) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	err := d.working.RemoveResource(r)
	if err != nil {
		return err
	}

	return d.save(r)
}

// failedToDestroy marks a resource whose destroy failed as destroy_failed and
// saves the working state. The resource is the saved copy, so it still holds
// the identity needed to try the destroy again.
func (d *destroyer) failedToDestroy(r any) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	meta.Status = types.StatusDestroyFailed

	return d.save(r)
}

// save persists the working state, it must be called with mu held
func (d *destroyer) save(r any) error {
	if d.store == nil {
		return nil
	}

	err := d.store.Save(d.working)
	if err != nil {
		id := ""
		if meta, metaErr := types.GetMeta(r); metaErr == nil {
			id = meta.ID
		}

		return fmt.Errorf("unable to save state after destroying %s: %w", id, err)
	}

	return nil
}
