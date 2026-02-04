package parser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// callPluginLifecycle calls the appropriate plugin lifecycle method for a resource
// This is only called when executePlugins=true (during Apply)
func (p *Parser) callPluginLifecycle(resource any, previousState *state.State) error {
	resourceMeta, err := types.GetMeta(resource)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase embedded: %w", err)
	}

	// Skip builtin resource types that don't have providers
	if resourceMeta.Type == resources.TypeVariable ||
		resourceMeta.Type == resources.TypeOutput ||
		resourceMeta.Type == resources.TypeModule ||
		resourceMeta.Type == resources.TypeRoot {
		return nil
	}

	// Get the provider for this resource
	if p.pluginRegistry == nil {
		return nil // No plugin registry configured
	}

	adapter := p.pluginRegistry.GetProviderForResource(resource)
	if adapter == nil {
		return nil // No provider for this resource type
	}

	ctx := context.Background()

	// Serialize current resource to JSON
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// Check if this resource existed in previous state
	var stateResource any
	if previousState != nil {
		stateResource, _ = previousState.FindResource(resourceMeta.ID)
	}

	if stateResource == nil {
		// Resource is new - call Create
		mutatedData, err := adapter.Create(ctx, resourceJSON)
		if err != nil {
			resourceMeta.Status = "failed"
			return fmt.Errorf("create failed for %s: %w", resourceMeta.ID, err)
		}

		// Unmarshal mutated data back into the resource object
		if mutatedData != nil {
			if err := json.Unmarshal(mutatedData, resource); err != nil {
				resourceMeta.Status = "failed"
				return fmt.Errorf("failed to unmarshal created resource: %w", err)
			}
		}

		resourceMeta.Status = "created"
	} else {
		// Resource exists - call Refresh, check Changed, then Update if needed

		// 1. Refresh to ensure state is up to date
		refreshedData, err := adapter.Refresh(ctx, resourceJSON)
		if err != nil {
			resourceMeta.Status = "failed"
			return fmt.Errorf("refresh failed for %s: %w", resourceMeta.ID, err)
		}

		// Unmarshal refreshed data back into the resource
		if refreshedData != nil {
			if err := json.Unmarshal(refreshedData, resource); err != nil {
				resourceMeta.Status = "failed"
				return fmt.Errorf("failed to unmarshal refreshed resource: %w", err)
			}
			// Re-serialize after refresh
			resourceJSON, err = json.Marshal(resource)
			if err != nil {
				return fmt.Errorf("failed to serialize resource after refresh: %w", err)
			}
		}

		// 2. Serialize state resource for comparison
		stateJSON, err := json.Marshal(stateResource)
		if err != nil {
			return fmt.Errorf("failed to serialize state resource: %w", err)
		}

		// 3. Check if resource has changed
		changed, err := adapter.Changed(ctx, stateJSON, resourceJSON)
		if err != nil {
			resourceMeta.Status = "failed"
			return fmt.Errorf("changed check failed for %s: %w", resourceMeta.ID, err)
		}

		// 4. If changed, call Update
		if changed {
			updatedData, err := adapter.Update(ctx, resourceJSON)
			if err != nil {
				resourceMeta.Status = "failed"
				return fmt.Errorf("update failed for %s: %w", resourceMeta.ID, err)
			}

			// Unmarshal updated data back into the resource
			if updatedData != nil {
				if err := json.Unmarshal(updatedData, resource); err != nil {
					resourceMeta.Status = "failed"
					return fmt.Errorf("failed to unmarshal updated resource: %w", err)
				}
			}

			resourceMeta.Status = "updated"
		} else {
			// No changes needed, preserve existing status
			stateMeta, err := types.GetMeta(stateResource)
			if err == nil && stateMeta.Status != "" {
				resourceMeta.Status = stateMeta.Status
			}
		}
	}

	return nil
}
