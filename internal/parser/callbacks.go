package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creasty/defaults"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/resources"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
	"github.com/jumppad-labs/xcl/internal/dag"
	"github.com/jumppad-labs/xcl/internal/cty"
)

// ProviderResolver resolves the provider adapter responsible for a given resource.
// Satisfied by *registry.PluginRegistry; exists so the walk callbacks can be tested
// against a mock instead of a real plugin registry.
type ProviderResolver interface {
	GetProviderForResource(resource any) plugins.ProviderAdapter
}

// TypeRegistry reports which resource types are plain Go types registered
// without a plugin. Resources of these types are handled like builtins, no
// provider is ever called for them. Satisfied by *registry.PluginRegistry.
type TypeRegistry interface {
	IsRegisteredType(name string) bool
}

// walkCallback creates the internal callback that is called when a node in the
// dag is visited. This callback is responsible for processing the resource and setting
// any linked values.
func walkCallback(parsedData *parsed, rp ResourceProvider, lifecycle *resourceLifecycle, options *ParserOptions, functions functionsForFile) func(v dag.Vertex) (diags dag.Diagnostics) {
	return func(v dag.Vertex) (diags dag.Diagnostics) {

		// v should be a resource (either builtin or schema-generated)
		r := v
		rMeta, err := types.GetMeta(r)
		if err != nil {
			return diags.Append(err)
		}

		// Skip the root node
		if rMeta.Type == resources.TypeRoot {
			lifecycle.progress.record(rMeta.ID, outcome{saved: r})
			return nil
		}

		// Skip disabled resources, resources could already be disabled if they are
		// part of a module that is disabled
		disabled, err := types.GetDisabled(r)
		if err != nil {
			panic(err) // This should never happen as we check this earlier
		}

		if disabled {
			lifecycle.progress.record(rMeta.ID, outcome{saved: r})
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
			lifecycle.progress.record(rMeta.ID, outcome{saved: r})
			return nil
		}

		// If there are defaults defined on the resource set them
		defaults.Set(r)

		// Decode the body into the resource
		diag := gohcl.DecodeBody(bdy, ctx, r)
		if diag.HasErrors() {
			pe := errors.NewParserErrorFromResource(
				r,
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
						fmt.Sprintf(`unable to evaluate 'variables' for module: %s`, valDiags.Error()),
					)
					return diags.Append(pe)
				}
				suppliedVars = val
			}

			mod.SubContext = &hcl.EvalContext{
				Functions: functions(rMeta.File),
				Variables: map[string]cty.Value{
					"variable": suppliedVars,
				},
			}
		}

		// Call provider lifecycle methods
		if err := lifecycle.apply(r); err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				fmt.Sprintf("provider lifecycle error: %s", err),
			)
			return diags.Append(pe)
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

// destroyWalkCallback creates the callback that destroys one resource during
// a destroy walk. Builtin, registered and disabled resources have no provider,
// they fire a destroy success event only. Every other resource is passed to
// its provider's Destroy as the saved copy. The outcome is reported to the
// destroyer, which removes a destroyed resource from the working state or
// keeps a failed one as destroy_failed, and saves the state. A failed save
// fails the step, so the resource's parents are never visited.
func destroyWalkCallback(d *destroyer) func(v dag.Vertex) (diags dag.Diagnostics) {
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

		resourceID := rMeta.ID
		rType := resourceType(rMeta)

		disabled, err := types.GetDisabled(r)
		if err != nil {
			return diags.Append(errors.NewParserErrorFromResource(r, err.Error()))
		}

		// Skip disabled, builtin and registered resource types, they never
		// reach a provider
		if disabled || handledWithoutProvider(d.types, rMeta.Type) {
			// Fire destroy events for provider-less types (always succeed with 0 time)
			fireParserEvent(d.options, "destroy", rType, resourceID, "success", 0, nil, nil)

			err := d.destroyed(r)
			if err != nil {
				return diags.Append(err)
			}

			return nil
		}

		// Get the provider for this resource
		adapter := d.resolver.GetProviderForResource(r)
		if adapter == nil {
			pe := errors.NewParserErrorFromResource(
				r,
				fmt.Sprintf("no provider found for resource type %s", rMeta.Type),
			)
			diags = diags.Append(pe)

			if saveErr := d.failedToDestroy(r); saveErr != nil {
				diags = diags.Append(saveErr)
			}

			return diags
		}

		// Serialize the resource to JSON for provider call
		resourceJSON, err := json.Marshal(r)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				fmt.Sprintf("failed to serialize resource for destroy: %s", err),
			)
			diags = diags.Append(pe)

			if saveErr := d.failedToDestroy(r); saveErr != nil {
				diags = diags.Append(saveErr)
			}

			return diags
		}

		// Call destroy on the provider
		fireParserEvent(d.options, "destroy", rType, resourceID, "start", 0, nil, resourceJSON)
		start := time.Now()
		err = adapter.Destroy(context.Background(), resourceJSON, false)
		duration := time.Since(start)

		if err != nil {
			fireParserEvent(d.options, "destroy", rType, resourceID, "error", duration, err, resourceJSON)

			pe := errors.NewParserErrorFromResource(
				r,
				fmt.Sprintf("destroy failed for %s: %s", resourceID, err),
			)
			diags = diags.Append(pe)

			if saveErr := d.failedToDestroy(r); saveErr != nil {
				diags = diags.Append(saveErr)
			}

			return diags
		}

		fireParserEvent(d.options, "destroy", rType, resourceID, "success", duration, nil, resourceJSON)

		err = d.destroyed(r)
		if err != nil {
			return diags.Append(err)
		}

		return nil
	}
}
