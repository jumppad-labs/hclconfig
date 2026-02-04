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

	for _, link := range rMeta.Links {
		// Parse the link into an FQDN
		fqdn, err := resources.ParseFQRN(link)
		if err != nil {
			continue // Skip invalid links
		}

		// Find the resource using findResource
		resource, ok := res.resources[fqdn.StringWithoutAttribute()]
		if !ok {
			continue // Skip if resource not found
		}

		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			panic(fmt.Sprintf("resource does not have ResourceBase: %v", err))
		}

		if fqdn.Type == resources.TypeVariable {
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

	ctx.Variables["variable"] = cty.ObjectVal(variableVars)

	// If this resource is in a module, also get the module's passed variables
	if rMeta.Module != "" {
		//modulePassedVars, err := getModuleVariables(c, rMeta.Module, functions)
		//if err == nil && !modulePassedVars.IsNull() {
		//	// Merge module passed variables with variable resources
		//	// Module passed variables override variable resource defaults
		//	mergedVars := make(map[string]cty.Value)

		//	// Start with variable resources
		//	for k, v := range variableVars {
		//		mergedVars[k] = v
		//	}

		//	// Override with module passed variables
		//	for k, v := range modulePassedVars.AsValueMap() {
		//		mergedVars[k] = v
		//	}

		//	ctx.Variables["variable"] = cty.ObjectVal(mergedVars)
		//}
	} else if options != nil {
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

	return ctx, nil
}
