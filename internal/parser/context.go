package parser

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/xcl/internal/convert"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/types"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// buildContextForResource creates a fresh context for a specific resource
// by building variables dynamically from config and module sources
func buildContextForResource(res *parsed, r any, options *ParserOptions, functions map[string]function.Function) (*hcl.EvalContext, error) {
	rMeta, err := types.GetMeta(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource metadata: %w", err)
	}

	ctx := &hcl.EvalContext{
		Functions: functions,
		Variables: map[string]cty.Value{},
	}

	// Initialize empty resource namespace
	ctx.Variables["resource"] = cty.ObjectVal(map[string]cty.Value{})

	// Get variables and resources that this resource actually depends on (from its links)
	variableVars := map[string]cty.Value{}
	resourceVars := map[string]cty.Value{}
	// moduleVars holds resources reached via a "module.<name>...." reference,
	// nested as moduleVars[moduleName][resourceType][resourceName] so that an
	// expression like module.consul_1.output.foo resolves against
	// ctx.Variables["module"]["consul_1"]["output"]["foo"].
	moduleVars := map[string]map[string]cty.Value{}

	for _, link := range rMeta.Links {
		// Parse the link into an FQDN
		fqdn, err := resources.ParseFQRN(link)
		if err != nil {
			continue // Skip invalid links
		}

		// Links are written with no knowledge of their parent module, so a
		// reference from inside a module (e.g. "variable.cpu_resources") must
		// be resolved relative to that module's own scope, matching how
		// getResourceDependencies resolves the same links when building the DAG.
		relFQDN := fqdn.AppendParentModule(rMeta.Module)

		// Find the resource using findResource
		resource, ok := res.resources[relFQDN.StringWithoutAttribute()]
		if !ok {
			continue // Skip if resource not found
		}

		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			panic(fmt.Sprintf("resource does not have ResourceBase: %v", err))
		}

		if fqdn.Type == resources.TypeVariable && fqdn.Module == "" {
			// Handle variable
			if variable, ok := resource.(*resources.Variable); ok {
				// Default is already a cty.Value
				variableVars[resourceMeta.Name] = variable.Default
			}
		} else {
			// Handle other resource types
			// Skip certain resource types that can't be safely converted to cty values
			if resourceMeta.Type == resources.TypeModule || resourceMeta.Type == resources.TypeRoot {
				continue
			}

			// Convert the resource to cty value
			var ctyRes cty.Value
			switch resourceMeta.Type {
			case resources.TypeOutput:
				out := resource.(*resources.Output)
				ctyRes = out.CtyValue
			case resources.TypeVariable:
				variable := resource.(*resources.Variable)
				ctyRes = variable.Default
			default:
				// For other resource types, convert the entire resource to cty
				ctyRes, err = convert.GoToCtyValue(resource)
				if err != nil {
					// If conversion fails, skip this resource
					continue
				}
			}

			// Skip null values - these resources haven't been processed yet
			if ctyRes.IsNull() {
				continue
			}

			// A reference written with a "module." prefix (fqdn.Module != "")
			// is nested under the module namespace instead of the flat
			// resource namespace, keyed by that reference's own module name.
			if fqdn.Module != "" {
				typeMap, ok := moduleVars[fqdn.Module]
				if !ok {
					typeMap = map[string]cty.Value{}
				}

				var innerMap map[string]cty.Value
				if existing, exists := typeMap[resourceMeta.Type]; exists && !existing.IsNull() {
					innerMap = make(map[string]cty.Value)
					for k, v := range existing.AsValueMap() {
						innerMap[k] = v
					}
				} else {
					innerMap = make(map[string]cty.Value)
				}

				innerMap[resourceMeta.Name] = ctyRes
				typeMap[resourceMeta.Type] = cty.ObjectVal(innerMap)
				moduleVars[fqdn.Module] = typeMap

				continue
			}

			// Add to the appropriate nested map structure
			var typeMap map[string]cty.Value
			if existingTypeVal, exists := resourceVars[resourceMeta.Type]; exists && !existingTypeVal.IsNull() {
				typeMap = make(map[string]cty.Value)
				for k, v := range existingTypeVal.AsValueMap() {
					typeMap[k] = v
				}
			} else {
				typeMap = make(map[string]cty.Value)
			}

			typeMap[resourceMeta.Name] = ctyRes
			resourceVars[resourceMeta.Type] = cty.ObjectVal(typeMap)
		}
	}

	// If this resource is in a module, merge in the module's passed variables
	// (module-supplied values override this resource's own variable defaults)
	if rMeta.Module != "" {
		owningModule, ok := res.resources["module."+rMeta.Module]
		if ok {
			if mod, ok := owningModule.(*resources.Module); ok && mod.SubContext != nil {
				if modVars, ok := mod.SubContext.Variables["variable"]; ok && !modVars.IsNull() {
					for k, v := range modVars.AsValueMap() {
						variableVars[k] = v
					}
				}
			}
		}
	}

	ctx.Variables["variable"] = cty.ObjectVal(variableVars)

	if rMeta.Module == "" && options != nil {
		// For root-level resources only, load variables from files and apply precedence
		// Precedence: variable defaults < .vars files < environment variables < direct variables

		// Load variables from .vars files (these override variable defaults)
		for _, vf := range options.VariablesFiles {
			if err := loadVariablesFromFile(ctx, vf); err != nil {
				// Continue processing other files even if one fails
				// This matches the behavior in parser.go
				continue
			}
		}

		// Apply environment variables and direct variables (these override .vars files)
		setVariables(ctx, options.Variables, options.VariableEnvPrefix)
	}

	// Set the resource variables in the context
	ctx.Variables["resource"] = cty.ObjectVal(resourceVars)

	// Set the module namespace, so references like
	// module.consul_1.output.foo resolve for resources outside that module
	moduleNamespace := map[string]cty.Value{}
	for moduleName, typeMap := range moduleVars {
		moduleNamespace[moduleName] = cty.ObjectVal(typeMap)
	}
	ctx.Variables["module"] = cty.ObjectVal(moduleNamespace)

	return ctx, nil
}
