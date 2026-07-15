package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creasty/defaults"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/silas/dag"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// walkCallback creates the internal callback that is called when a node in the
// dag is visited. This callback is responsible for processing the resource and setting
// any linked values. The executePlugins parameter controls whether provider lifecycle methods are called.
func walkCallback(parsedData *parsed, previousParsed *parsed, rp ResourceProvider, registry *registry.PluginRegistry, options *ParserOptions, functions map[string]function.Function, executePlugins bool) func(v dag.Vertex) (diags dag.Diagnostics) {
	return func(v dag.Vertex) (diags dag.Diagnostics) {

		// v should be a resource (either builtin or schema-generated)
		r := v
		rMeta, err := types.GetMeta(r)
		if err != nil {
			return diags.Append(err)
		}

		// Skip the root node
		if rMeta.Type == resources.TypeRoot {
			return nil
		}

		// Skip disabled resources, resources could already be disabled if they are
		// part of a module that is disabled
		disabled, err := types.GetDisabled(r)
		if err != nil {
			panic(err) // This should never happen as we check this earlier
		}

		if disabled {
			return nil
		}

		// get the body of the resource so that we can decode it
		// this is stored when we original parsed the file containing the resource
		bdy, ok := parsedData.bodies[rMeta.ID]
		if !ok {
			panic(fmt.Sprintf(`no body found for resource "%s"`, rMeta.ID))
		}

		// Build a fresh context for this resource dynamically
		ctx, err := buildContextForResource(parsedData, r, options, functions)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("failed to build resource context: %s", err),
			)
			return diags.Append(pe)
		}

		// A resource can be defined as disabled by an expression, we need to
		// evaluate this expression to determine if the resource should be processed.
		// If we do not do this first we might attempt to interpolate a value on a
		// resource that does not exist, or is disabled.
		isDisabled, err := processDisabled(bdy, ctx, r)
		if err != nil {
			return diags.Append(err)
		}

		// If the type is a module and the module is disabled, we need to
		// set all the resources in the module to disabled.
		if isDisabled && rMeta.Type == resources.TypeModule {
			// Find all dependent resources for this module. Ignore the error:
			// a module with no remaining resources returns a not-found error,
			// which is not a failure here.
			dr, _ := rp.FindModuleResources(rMeta.ID, true)

			// Set all the dependents to disabled
			for _, d := range dr {
				types.SetDisabled(d, true)
			}
		}

		// If the resource is disabled we need to skip the resource
		if isDisabled {
			return nil
		}

		// If there are defaults defined on the resource set them
		defaults.Set(r)

		// Decode the body into the resource
		diag := gohcl.DecodeBody(bdy, ctx, r)
		if diag.HasErrors() {
			// Check the error types and determine if we should set a warning or error
			level := checkIfErrorInFunction(diag)
			pe := errors.NewParserErrorFromResource(
				r,
				level,
				fmt.Sprintf(`unable to decode body: %s`, diag.Error()),
			)

			return diags.Append(pe)
		}

		// If this is a module, evaluate its "variables" attribute (not HCL-tag
		// decoded - gocty's implied-type decode can't represent a heterogeneous
		// object as a single Go type) and store the result on SubContext, where
		// buildContextForResource will find and merge it over each child's own
		// variable defaults. The module's children haven't decoded yet at this
		// point (DAG order runs the module vertex itself first), so SubContext
		// can only hold the module's own supplied overrides here, not a full
		// defaults+overrides merge - each child's own defaults are already
		// resolved independently by buildContextForResource from that child's
		// own dependency links.
		if rMeta.Type == resources.TypeModule {
			mod := r.(*resources.Module)

			suppliedVars := cty.EmptyObjectVal
			if mod.Variables != nil {
				val, valDiags := mod.Variables.Value(ctx)
				if valDiags.HasErrors() {
					pe := errors.NewParserErrorFromResource(
						r,
						errors.ParserErrorLevelError,
						fmt.Sprintf(`unable to evaluate 'variables' for module: %s`, valDiags.Error()),
					)
					return diags.Append(pe)
				}
				suppliedVars = val
			}

			mod.SubContext = &hcl.EvalContext{
				Functions: functions,
				Variables: map[string]cty.Value{
					"variable": suppliedVars,
				},
			}
		}

		// Call provider lifecycle methods if executePlugins is true
		if executePlugins {
			if err := callProviderLifecycle(r, previousParsed, registry, options); err != nil {
				pe := errors.NewParserErrorFromResource(
					r,
					errors.ParserErrorLevelError,
					fmt.Sprintf("provider lifecycle error: %s", err),
				)
				return diags.Append(pe)
			}
		}

		// Convert CtyValue to Value for output and local resources
		switch rMeta.Type {
		case resources.TypeOutput:
			out := r.(*resources.Output)
			if !out.CtyValue.IsNull() {
				out.Value = convertCtyToGo(out.CtyValue)
			}
		}

		return nil
	}
}

// destroyWalkCallback creates a simplified callback for destroying resources
// Skips complex processing since resources are already fully processed
func destroyWalkCallback(registry *registry.PluginRegistry, options *ParserOptions) func(v dag.Vertex) (diags dag.Diagnostics) {
	return func(v dag.Vertex) (diags dag.Diagnostics) {
		// v should be a resource (either builtin or schema-generated)
		r := v

		// Skip the destroy root node
		rMeta, err := types.GetMeta(r)
		if err != nil {
			return nil // Skip resources without ResourceBase
		}
		if rMeta.Type == resources.TypeRoot {
			return nil
		}

		// Skip builtin resource types that don't have providers
		if rMeta.Type == resources.TypeVariable ||
			rMeta.Type == resources.TypeOutput ||
			rMeta.Type == resources.TypeModule ||
			rMeta.Type == resources.TypeRoot {

			// Fire destroy events for builtin types (always succeed with 0 time)
			resourceType := fmt.Sprintf("%s.%s", rMeta.Type, rMeta.Name)
			fireParserEvent(options, "destroy", resourceType, rMeta.ID, "success", 0, nil, nil)

			rMeta.Status = "destroyed"

			return nil
		}

		// Get the provider for this resource
		adapter := registry.GetProviderForResource(r)
		if adapter == nil {

			rMeta.Status = "destroyed_failed"

			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("no provider found for resource type %s", rMeta.Type),
			)
			return diags.Append(pe)
		}

		ctx := context.Background()
		resourceID := rMeta.ID
		resourceType := fmt.Sprintf("%s.%s", rMeta.Type, rMeta.Name)

		// Serialize the resource to JSON for provider call
		resourceJSON, err := json.Marshal(r)
		if err != nil {
			rMeta.Status = "destroyed_failed"

			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("failed to serialize resource for destroy: %s", err),
			)
			return diags.Append(pe)
		}

		// Call destroy on the provider
		fireParserEvent(options, "destroy", resourceType, resourceID, "start", 0, nil, resourceJSON)
		start := time.Now()
		err = adapter.Destroy(ctx, resourceJSON, false)
		duration := time.Since(start)

		if err != nil {
			fireParserEvent(options, "destroy", resourceType, resourceID, "error", duration, err, resourceJSON)
			rMeta.Status = "destroy_failed"

			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("destroy failed: %s", err),
			)
			return diags.Append(pe)
		}

		fireParserEvent(options, "destroy", resourceType, resourceID, "success", duration, nil, resourceJSON)
		rMeta.Status = "destroyed"

		return nil
	}
}

// callProviderLifecycle calls the appropriate provider lifecycle method based on resource state
func callProviderLifecycle(r any, previousParsed *parsed, registry *registry.PluginRegistry, options *ParserOptions) error {
	rMeta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	// Skip builtin resource types that don't have providers
	if rMeta.Type == resources.TypeVariable ||
		rMeta.Type == resources.TypeOutput ||
		rMeta.Type == resources.TypeModule ||
		rMeta.Type == resources.TypeRoot {
		return nil
	}

	// Get the provider for this resource
	adapter := registry.GetProviderForResource(r)
	if adapter == nil {
		return fmt.Errorf("no provider found for resource type %s", rMeta.Type)
	}

	ctx := context.Background()
	resourceID := rMeta.ID
	resourceType := fmt.Sprintf("%s.%s", rMeta.Type, rMeta.Name)

	// Serialize the current resource to JSON for provider calls
	resourceJSON, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// Check if this resource existed in previous state
	var previousResource any
	if previousParsed != nil {
		previousResource = previousParsed.resources[rMeta.ID]
	}

	// Determine lifecycle operation based on previous state
	if previousResource == nil {
		// New resource - call Create
		fireParserEvent(options, "create", resourceType, resourceID, "start", 0, nil, resourceJSON)
		start := time.Now()
		updatedJSON, err := adapter.Create(ctx, resourceJSON)
		duration := time.Since(start)

		if err != nil {
			fireParserEvent(options, "create", resourceType, resourceID, "error", duration, err, resourceJSON)
			rMeta.Status = "failed"
			return fmt.Errorf("create failed: %w", err)
		}

		// Update the resource with the result from the provider
		if len(updatedJSON) > 0 {
			if err := json.Unmarshal(updatedJSON, r); err != nil {
				return fmt.Errorf("failed to unmarshal created resource: %w", err)
			}
		}

		fireParserEvent(options, "create", resourceType, resourceID, "success", duration, nil, resourceJSON)
		rMeta.Status = "created"
	} else {
		// Existing resource - check if changed
		previousResourceJSON, err := json.Marshal(previousResource)
		if err != nil {
			return fmt.Errorf("failed to serialize previous resource: %w", err)
		}

		// Call Refresh to get current state from provider
		fireParserEvent(options, "refresh", resourceType, resourceID, "start", 0, nil, resourceJSON)
		start := time.Now()
		refreshedJSON, err := adapter.Refresh(ctx, resourceJSON)
		duration := time.Since(start)

		if err != nil {
			fireParserEvent(options, "refresh", resourceType, resourceID, "error", duration, err, resourceJSON)
			// Continue even if refresh fails
		} else {
			// Update the resource with refreshed state
			if len(refreshedJSON) > 0 {
				if err := json.Unmarshal(refreshedJSON, r); err != nil {
					return fmt.Errorf("failed to unmarshal refreshed resource: %w", err)
				}
			}
			fireParserEvent(options, "refresh", resourceType, resourceID, "success", duration, nil, resourceJSON)
		}

		// Check if resource has changed
		fireParserEvent(options, "changed", resourceType, resourceID, "start", 0, nil, resourceJSON)
		start = time.Now()
		changed, err := adapter.Changed(ctx, previousResourceJSON, resourceJSON)
		duration = time.Since(start)

		if err != nil {
			fireParserEvent(options, "changed", resourceType, resourceID, "error", duration, err, resourceJSON)
			return fmt.Errorf("changed check failed: %w", err)
		}

		fireParserEvent(options, "changed", resourceType, resourceID, "success", duration, nil, resourceJSON)

		if changed {
			// Resource changed - call Update
			fireParserEvent(options, "update", resourceType, resourceID, "start", 0, nil, resourceJSON)
			start = time.Now()
			updatedJSON, err := adapter.Update(ctx, resourceJSON)
			duration = time.Since(start)

			if err != nil {
				fireParserEvent(options, "update", resourceType, resourceID, "error", duration, err, resourceJSON)
				rMeta.Status = "failed"
				return fmt.Errorf("update failed: %w", err)
			}

			// Update the resource with the result from the provider
			if len(updatedJSON) > 0 {
				if err := json.Unmarshal(updatedJSON, r); err != nil {
					return fmt.Errorf("failed to unmarshal updated resource: %w", err)
				}
			}

			fireParserEvent(options, "update", resourceType, resourceID, "success", duration, nil, resourceJSON)
			rMeta.Status = "updated"
		} else {
			// Resource unchanged - preserve existing status
			if previousMeta, err := types.GetMeta(previousResource); err == nil {
				rMeta.Status = previousMeta.Status
			}
		}
	}

	return nil
}
