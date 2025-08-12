package hclconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creasty/defaults"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/jumppad-labs/hclconfig/errors"
	"github.com/jumppad-labs/hclconfig/internal/convert"
	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/silas/dag"
	"github.com/zclconf/go-cty/cty"
)

// doYaLikeDAGs? dags? yeah dags! oh dogs.
// https://www.youtube.com/watch?v=ZXILzUpVx7A&t=0s
func doYaLikeDAGs(c *Config) (*dag.AcyclicGraph, error) {
	// create root node

	graph := &dag.AcyclicGraph{}

	// add a root node for the graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, "root")
	graph.Add(root)

	// Loop over all resources and add to graph
	for _, resource := range c.Resources {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}

		// use a map to keep a unique list

		// add links to dependencies
		for _, d := range resourceMeta.Links {
			err := types.AppendUniqueDependency(resource, d)
			if err != nil {
				pe := errors.NewParserErrorFromResource(
					resource,
					errors.ParserErrorLevelError,
					fmt.Sprintf("unable to append dependency: %s, error: %s", d, err),
				)
				return nil, pe
			}
		}

		deps, err := types.GetDependencies(resource)
		if err != nil {
			return nil, fmt.Errorf("failed to get dependencies for resource %s: %w", resourceMeta.ID, err)
		}

		// create a map to keep track of unique dependencies
		// a map is easier than a slice for this purpose
		// as with a slice we would have to check if the dependency
		// already exists before adding it
		dependencies := make(map[any]bool)

		for _, d := range deps {
			var err error
			fqdn, err := resources.ParseFQRN(d)
			if err != nil {
				pe := errors.NewParserErrorFromResource(
					resource,
					errors.ParserErrorLevelError,
					fmt.Sprintf("invalid dependency: %s, error: %s", d, err),
				)
				return nil, pe
			}

			// when the dependency is a module, depend on all resources in the module
			if fqdn.Type == resources.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resourceMeta.Module)

				// we ignore the error here as it may be possible that the module depends on
				// disabled resources
				deps, _ := c.FindModuleResources(relFQDN.String(), true)

				for _, dep := range deps {
					dependencies[dep] = true
				}
			} else {
				// when the dependency is a resource, depend on the resource
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resourceMeta.Module)

				// we ignore the error here as it may be possible that the module depends on
				// disabled resources
				dep, _ := c.FindResource(relFQDN.String())

				dependencies[dep] = true
			}
		}

		// if this resource is part of a module make it depend on that module
		if resourceMeta.Module != "" {
			fqdnString := fmt.Sprintf("module.%s", resourceMeta.Module)

			d, err := c.FindResource(fqdnString)
			if err != nil {
				pe := errors.NewParserErrorFromResource(
					resource,
					errors.ParserErrorLevelError,
					fmt.Sprintf("unable to find parent module: '%s', error: %s", fqdnString, err),
				)
				return nil, pe
			}

			hasDeps = true
			dependencies[d] = true
		}

		for d := range dependencies {
			hasDeps = true
			//fmt.Println("connect", resource.Metadata().ID, "to", d.Metadata().ID)
			graph.Connect(dag.BasicEdge(d, resource))
		}

		// if no deps add to root node
		if !hasDeps {
			//fmt.Println("connect", resource.Metadata().ID, "to root")
			graph.Connect(dag.BasicEdge(root, resource))
		}
	}

	return graph, nil
}

// buildDestroyDAG creates a DAG for destroying resources with reversed dependencies
// Resources with dependencies must be destroyed before their dependencies
func buildDestroyDAG(toDestroy []any) (*dag.AcyclicGraph, error) {
	graph := &dag.AcyclicGraph{}

	if len(toDestroy) == 0 {
		return graph, nil
	}

	// Add a root node for the destroy graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, "destroy_root")
	graph.Add(root)

	// Add all resources to be destroyed to the graph
	for _, resource := range toDestroy {
		graph.Add(resource)
	}

	// Create a map for quick lookup of resources in destroy list
	destroyMap := make(map[string]any)
	for _, resource := range toDestroy {
		meta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		destroyMap[meta.ID] = resource
	}

	// Add REVERSED dependencies between resources to be destroyed
	// If A depends on B, we want to destroy A before B, so we create edge A -> B
	resourcesWithDeps := make(map[any]bool)

	for _, resource := range toDestroy {
		// Collect all dependencies for this resource
		var allDependencies []string

		// Add explicit dependencies from depends_on
		deps, err := types.GetDependencies(resource)
		if err == nil {
			allDependencies = append(allDependencies, deps...)
		}

		// Add implicit dependencies from resource links (interpolations)
		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		allDependencies = append(allDependencies, resourceMeta.Links...)

		hasDepsInDestroyList := false

		// For each dependency, if it's also being destroyed, create a dependency edge
		for _, dependency := range allDependencies {
			if dependency == "" {
				continue
			}

			// Check if the dependency is also in the destroy list
			if depResource, exists := destroyMap[dependency]; exists {
				// Create edge: resource -> depResource (destroy resource before depResource)
				graph.Connect(dag.BasicEdge(resource, depResource))
				resourcesWithDeps[resource] = true
				resourcesWithDeps[depResource] = true
				hasDepsInDestroyList = true
			}
		}

		// If this resource has no dependencies in the destroy list, connect it to root
		if !hasDepsInDestroyList {
			graph.Connect(dag.BasicEdge(root, resource))
		}
	}

	// Connect all resources without dependencies to root
	for _, resource := range toDestroy {
		hasIncomingEdges := false
		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}

		// Check if this resource has any incoming edges from other destroy resources
		for _, otherResource := range toDestroy {
			if otherResource == resource {
				continue
			}

			// Check if otherResource depends on this resource
			otherResourceMeta, err := types.GetMeta(otherResource)
			if err != nil {
				continue
			}
			otherDeps, _ := types.GetDependencies(otherResource)
			otherDeps = append(otherDeps, otherResourceMeta.Links...)
			for _, dep := range otherDeps {
				if dep == resourceMeta.ID {
					hasIncomingEdges = true
					break
				}
			}
			if hasIncomingEdges {
				break
			}
		}

		// If no incoming edges from destroy resources, connect to root
		if !hasIncomingEdges {
			graph.Connect(dag.BasicEdge(root, resource))
		}
	}

	return graph, nil
}

// destroyWalkCallback creates a simplified callback for destroying resources
// Skips complex processing since resources are already fully processed
func destroyWalkCallback(registry *PluginRegistry, options *ParserOptions) func(v dag.Vertex) (diags dag.Diagnostics) {
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

// buildContextForResource creates a fresh context for a specific resource
// by building variables dynamically from config and module sources
func buildContextForResource(c *Config, r any, options *ParserOptions, functions map[string]function.Function) (*hcl.EvalContext, error) {
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
		resource, err := c.findResource(fqdn.StringWithoutAttribute())
		if err != nil {
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
		modulePassedVars, err := getModuleVariables(c, rMeta.Module, functions)
		if err == nil && !modulePassedVars.IsNull() {
			// Merge module passed variables with variable resources
			// Module passed variables override variable resource defaults
			mergedVars := make(map[string]cty.Value)
			
			// Start with variable resources
			for k, v := range variableVars {
				mergedVars[k] = v
			}
			
			// Override with module passed variables
			for k, v := range modulePassedVars.AsValueMap() {
				mergedVars[k] = v
			}
			
			ctx.Variables["variable"] = cty.ObjectVal(mergedVars)
		}
	} else if options != nil {
		// For root-level resources only, load variables from files and apply precedence
		// Precedence: variable defaults < .vars files < environment variables < direct variables
		
		// Create a parser instance to access the helper methods
		p := &Parser{options: *options}
		
		// Load variables from .vars files (these override variable defaults)
		for _, vf := range options.VariablesFiles {
			if err := p.loadVariablesFromFile(ctx, vf); err != nil {
				// Continue processing other files even if one fails
				// This matches the behavior in parser.go
				continue
			}
		}
		
		// Apply environment variables and direct variables (these override .vars files)
		p.setVariables(ctx, options.Variables)
	}

	// Set the resource variables in the context
	ctx.Variables["resource"] = cty.ObjectVal(resourceVars)

	return ctx, nil
}

// getModuleVariables finds a module by name and returns its variables as a cty.Value
func getModuleVariables(c *Config, moduleName string, functions map[string]function.Function) (cty.Value, error) {
	// Find the module resource
	moduleResource, err := c.FindResource(fmt.Sprintf("resource.module.%s", moduleName))
	if err != nil {
		return cty.NullVal(cty.DynamicPseudoType), fmt.Errorf("module %s not found: %w", moduleName, err)
	}

	// Get the module's body to access the variables attribute
	moduleBody, err := c.getBody(moduleResource)
	if err != nil {
		return cty.NullVal(cty.DynamicPseudoType), fmt.Errorf("failed to get module body: %w", err)
	}

	// Check if the module has a variables attribute
	if moduleBody.Attributes["variables"] == nil {
		return cty.ObjectVal(map[string]cty.Value{}), nil // No variables to process
	}

	// Build context with root variables for evaluating module variables expression
	basicCtx := &hcl.EvalContext{
		Functions: functions,
		Variables: map[string]cty.Value{},
	}
	
	// Add root-level variables to context so module can reference them
	// For now, only handle simple literal values and skip HCL expression evaluation
	rootVariableVars := map[string]cty.Value{}
	for _, resource := range c.Resources {
		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		// Only include variable resources that are in the root (no module)
		if resourceMeta.Type == resources.TypeVariable && resourceMeta.Module == "" {
			if variable, ok := resource.(*resources.Variable); ok {
				// Default is already a cty.Value
				if !variable.Default.IsNull() {
					rootVariableVars[resourceMeta.Name] = variable.Default
				}
			}
		}
	}
	
	basicCtx.Variables["variable"] = cty.ObjectVal(rootVariableVars)

	varsVal, diags := moduleBody.Attributes["variables"].Expr.Value(basicCtx)
	if diags.HasErrors() {
		return cty.NullVal(cty.DynamicPseudoType), fmt.Errorf("failed to evaluate module variables: %s", diags.Error())
	}

	return varsVal, nil
}

// walkCallback creates the internal callback that is called when a node in the
// dag is visited. This callback is responsible for processing the resource and setting
// any linked values
func walkCallback(c *Config, previousState *Config, registry *PluginRegistry, options *ParserOptions, functions map[string]function.Function) func(v dag.Vertex) (diags dag.Diagnostics) {

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

		// Debug: print processing order
		fmt.Printf("Processing: %s %s\n", rMeta.Type, rMeta.Name)

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
		bdy, err := c.getBody(r)
		if err != nil {
			panic(fmt.Sprintf(`no body found for resource "%s"`, rMeta.ID))
		}

		// Build a fresh context for this resource dynamically
		ctx, err := buildContextForResource(c, r, options, functions)
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
			// Find all dependent resources for this module
			dr, err := c.FindModuleResources(rMeta.ID, true)
			if err != nil {
				// Should not be here unless an internal error so hard fail
				panic(err)
			}

			// Set all the dependents to disabled
			for _, d := range dr {
				types.SetDisabled(d, true)
				fmt.Println("DEBUG: Setting resource", d, "to disabled")
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

		// If this is a module, process its variables and update sub-resource contexts
		if rMeta.Type == resources.TypeModule {
			if err := processModuleVariables(c, r, ctx); err != nil {
				pe := errors.NewParserErrorFromResource(
					r,
					errors.ParserErrorLevelError,
					fmt.Sprintf("failed to process module variables: %s", err),
				)
				return diags.Append(pe)
			}
		}

		if err := callProviderLifecycle(r, previousState, registry, options); err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
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

// fireParserEvent fires a parser event if the callback is configured
func fireParserEvent(options *ParserOptions, operation, resourceType, resourceID, phase string, duration time.Duration, err error, data []byte) {
	if options != nil && options.OnParserEvent != nil {
		event := ParserEvent{
			Operation:    operation,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Phase:        phase,
			Duration:     duration,
			Error:        err,
			Data:         data,
		}
		options.OnParserEvent(event)
	}
}

// processDisabled processes any expression for the disabled attribute
// and sets the disabled state on the resource
func processDisabled(bdy *hclsyntax.Body, ctx *hcl.EvalContext, r dag.Vertex) (bool, error) {
	var isDisabled bool

	// This expression could be a reference to another resource or it could be a
	// function or a conditional statement. We need to evaluate the expression
	// to determine if the resource should be disabled
	if attr, ok := bdy.Attributes["disabled"]; ok {
		// now we need to evaluate the expression
		expdiags := gohcl.DecodeExpression(attr.Expr, ctx, &isDisabled)
		if expdiags.HasErrors() {
			return isDisabled,
				errors.NewParserErrorFromResource(
					r,
					errors.ParserErrorLevelError,
					fmt.Sprintf("unable to decode disabled expression: %s", expdiags.Error()),
				)
		}

		err := types.SetDisabled(r, isDisabled)
		if err != nil {
			return isDisabled, errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("failed to set disabled state: %s", err),
			)
		}
	}

	return isDisabled, nil
}

func checkIfErrorInFunction(diag hcl.Diagnostics) string {
	level := errors.ParserErrorLevelWarning

	for _, e := range diag.Errs() {
		err, ok := e.(*hcl.Diagnostic)
		if !ok {
			continue
		}

		if err.Summary == "Error in function call" {
			level = errors.ParserErrorLevelError
			break
		}
	}
	return level
}

// processModuleVariables processes a module's variables and updates the contexts of all sub-resources
func processModuleVariables(c *Config, moduleResource any, sharedCtx *hcl.EvalContext) error {
	moduleMeta, err := types.GetMeta(moduleResource)
	if err != nil {
		return fmt.Errorf("failed to get module metadata: %w", err)
	}

	// Get the module's body to access the variables attribute
	moduleBody, err := c.getBody(moduleResource)
	if err != nil {
		return fmt.Errorf("failed to get module body: %w", err)
	}

	// Check if the module has a variables attribute
	if moduleBody.Attributes["variables"] == nil {
		return nil // No variables to process
	}

	// Evaluate the variables expression using the shared context
	varsVal, diags := moduleBody.Attributes["variables"].Expr.Value(sharedCtx)
	if diags.HasErrors() {
		return fmt.Errorf("failed to evaluate module variables: %s", diags.Error())
	}

	if varsVal.IsNull() || !varsVal.Type().IsObjectType() {
		return nil // No variables to process
	}

	// Get all resources in this module
	moduleResources, err := c.FindModuleResources(moduleMeta.ID, true)
	if err != nil {
		return fmt.Errorf("failed to find module resources: %w", err)
	}

	// Update the context of each module resource
	for _, moduleRes := range moduleResources {
		storedCtx, err := c.getContext(moduleRes)
		if err != nil || storedCtx == nil {
			continue // Skip resources without stored contexts
		}

		// Ensure the stored context has a Variables map
		if storedCtx.Variables == nil {
			storedCtx.Variables = make(map[string]cty.Value)
		}

		// Create or update the variable namespace
		varMap := make(map[string]cty.Value)
		if existing, ok := storedCtx.Variables["variable"]; ok && !existing.IsNull() {
			for k, v := range existing.AsValueMap() {
				varMap[k] = v
			}
		}

		// Add the module variables
		for k, v := range varsVal.AsValueMap() {
			varMap[k] = v
		}

		// Update the variable namespace
		storedCtx.Variables["variable"] = cty.ObjectVal(varMap)
	}

	return nil
}

// callProviderLifecycle calls the appropriate provider lifecycle methods for a resource
func callProviderLifecycle(resource any, previousState *Config, registry *PluginRegistry, options *ParserOptions) error {

	resourceMeta, err := types.GetMeta(resource)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase embedded: %w", err)
	}

	// Skip builtin resource types that don't have providers, but fire events for them
	if resourceMeta.Type == resources.TypeVariable ||
		resourceMeta.Type == resources.TypeOutput ||
		resourceMeta.Type == resources.TypeModule ||
		resourceMeta.Type == resources.TypeRoot {

		// Fire events for all builtin types (always succeed with 0 time)
		// Note: Variables are also handled in parseVariablesInFile, but this catches any that go through DAG
		resourceType := fmt.Sprintf("%s.%s", resourceMeta.Type, resourceMeta.Name)
		fireParserEvent(options, "create", resourceType, resourceMeta.ID, "success", 0, nil, nil)

		return nil
	}

	// Get the provider for this resource
	adapter := registry.GetProviderForResource(resource)
	if adapter == nil {
		// No provider found - this might be a builtin type without a provider
		return fmt.Errorf("no provider found for resource type %s", resourceMeta.Type)
	}

	ctx := context.Background()
	resourceID := resourceMeta.ID
	resourceType := fmt.Sprintf("%s.%s", resourceMeta.Type, resourceMeta.Name)

	// Serialize the current resource to JSON
	currentJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// Check if resource exists in state
	var stateResource any
	var existsInState bool
	if previousState != nil {
		var err error
		stateResource, err = previousState.FindResource(resourceID)
		existsInState = (err == nil)
	}

	if existsInState {
		// Resource exists - follow the lifecycle: Refresh -> Changed -> Update/Skip

		// 1. Call Refresh to ensure state is up to date
		fireParserEvent(options, "refresh", resourceType, resourceID, "start", 0, nil, currentJSON)
		start := time.Now()
		refreshedData, err := adapter.Refresh(ctx, currentJSON)
		duration := time.Since(start)

		if err != nil {
			fireParserEvent(options, "refresh", resourceType, resourceID, "error", duration, err, currentJSON)
			resourceMeta.Status = "failed"
			return fmt.Errorf("refresh failed: %w", err)
		}

		// Unmarshal refreshed data back into the resource object to preserve provider mutations
		if refreshedData != nil {
			if err := json.Unmarshal(refreshedData, resource); err != nil {
				fireParserEvent(options, "refresh", resourceType, resourceID, "error", duration, err, currentJSON)
				resourceMeta.Status = "failed"
				return fmt.Errorf("failed to unmarshal refreshed resource: %w", err)
			}
		}

		fireParserEvent(options, "refresh", resourceType, resourceID, "success", duration, nil, currentJSON)

		// 2. Serialize state resource for comparison
		stateJSON, err := json.Marshal(stateResource)
		if err != nil {
			return fmt.Errorf("failed to serialize state resource: %w", err)
		}

		// 3. Check if resource has changed
		fireParserEvent(options, "changed", resourceType, resourceID, "start", 0, nil, currentJSON)
		start = time.Now()
		changed, err := adapter.Changed(ctx, stateJSON, currentJSON)
		duration = time.Since(start)

		if err != nil {
			fireParserEvent(options, "changed", resourceType, resourceID, "error", duration, err, currentJSON)
			resourceMeta.Status = "failed"
			return fmt.Errorf("changed check failed: %w", err)
		}
		fireParserEvent(options, "changed", resourceType, resourceID, "success", duration, nil, currentJSON)

		// 4. If changed, call Update
		if changed {
			fireParserEvent(options, "update", resourceType, resourceID, "start", 0, nil, currentJSON)
			start = time.Now()
			updatedData, err := adapter.Update(ctx, currentJSON)
			duration = time.Since(start)

			if err != nil {
				fireParserEvent(options, "update", resourceType, resourceID, "error", duration, err, currentJSON)
				resourceMeta.Status = "failed"
				return fmt.Errorf("update failed: %w", err)
			}

			// Unmarshal updated data back into the resource object to preserve provider mutations
			if updatedData != nil {
				if err := json.Unmarshal(updatedData, resource); err != nil {
					fireParserEvent(options, "update", resourceType, resourceID, "error", duration, err, currentJSON)
					resourceMeta.Status = "failed"
					return fmt.Errorf("failed to unmarshal updated resource: %w", err)
				}
			}

			fireParserEvent(options, "update", resourceType, resourceID, "success", duration, nil, currentJSON)
			resourceMeta.Status = "updated"
		} else {
			// No changes needed, preserve existing status
			// TODO, I don't think this code is correct, needs investigation
			stateMeta, err := types.GetMeta(stateResource)
			if err == nil && stateMeta.Status != "" {
				resourceMeta.Status = stateMeta.Status
			}
		}
	} else {
		// Resource doesn't exist in state - create it
		fireParserEvent(options, "create", resourceType, resourceID, "start", 0, nil, currentJSON)
		start := time.Now()
		mutatedData, err := adapter.Create(ctx, currentJSON)
		duration := time.Since(start)

		if err != nil {
			fireParserEvent(options, "create", resourceType, resourceID, "error", duration, err, currentJSON)
			resourceMeta.Status = "failed"
			return fmt.Errorf("create failed: %w", err)
		}

		// Unmarshal mutated data back into the resource object to preserve provider mutations
		if mutatedData != nil {
			if err := json.Unmarshal(mutatedData, resource); err != nil {
				fireParserEvent(options, "create", resourceType, resourceID, "error", duration, err, currentJSON)
				resourceMeta.Status = "failed"
				return fmt.Errorf("failed to unmarshal mutated resource: %w", err)
			}
		}

		fireParserEvent(options, "create", resourceType, resourceID, "success", duration, nil, currentJSON)
		resourceMeta.Status = "created"
	}

	return nil
}

// setContextVariablesFromList sets the context variables from a list of resource links
//
// for example: given the values ["module.module1.module2.resource.container.mine.id"]
// the context variable "module.module1.module2.resource.container.mine.id" will be set to the
// value defined by the resource of type container with the name mine and the attribute id

// convertCtyToGo recursively converts a cty.Value to a Go value
func convertCtyToGo(val cty.Value) any {
	if val.IsNull() {
		return nil
	}

	switch {
	case val.Type() == cty.String:
		return val.AsString()
	case val.Type() == cty.Number:
		f, _ := val.AsBigFloat().Float64()
		return f
	case val.Type() == cty.Bool:
		return val.True()
	case val.Type().IsListType() || val.Type().IsTupleType():
		slice := []any{} // Initialize as empty slice, not nil slice
		for it := val.ElementIterator(); it.Next(); {
			_, elem := it.Element()
			slice = append(slice, convertCtyToGo(elem)) // Recursive call
		}
		return slice
	case val.Type().IsObjectType() || val.Type().IsMapType():
		objMap := make(map[string]any)
		for it := val.ElementIterator(); it.Next(); {
			key, elem := it.Element()
			keyStr := key.AsString()
			objMap[keyStr] = convertCtyToGo(elem) // Recursive call
		}
		return objMap
	default:
		// For unknown types, try to get a meaningful representation
		// This could be further improved based on specific cty types
		return val.GoString()
	}
}
