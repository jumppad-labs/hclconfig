package hclconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/creasty/defaults"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

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
		// ignore variables (referenceable but don't need DAG processing)
		// providers are now included in DAG to support resource dependencies
		if resource.Metadata().Type != resources.TypeVariable {
			graph.Add(resource)
		}
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		// do nothing with variables (referenceable but don't need DAG processing)
		if resource.Metadata().Type == resources.TypeVariable {
			continue
		}

		// use a map to keep a unique list
		dependencies := map[types.Resource]bool{}

		// add links to dependencies
		// this is here for now as we might need to process these two lists separately
		// resource.SetDependsOn(append(resource.GetDependsOn(), resource.Metadata().Links...))
		for _, d := range resource.Metadata().Links {
			resource.AddDependency(d)
		}

		for _, d := range resource.GetDependencies() {
			var err error
			fqdn, err := resources.ParseFQRN(d)
			if err != nil {
				pe := &errors.ParserError{}
				pe.Line = resource.Metadata().Line
				pe.Column = resource.Metadata().Column
				pe.Filename = resource.Metadata().File
				pe.Message = fmt.Sprintf("invalid dependency: %s, error: %s", d, err)
				pe.Level = errors.ParserErrorLevelError

				return nil, pe
			}

			// when the dependency is a module, depend on all resources in the module
			if fqdn.Type == resources.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resource.Metadata().Module)

				// we ignore the error here as it may be possible that the module depends on
				// disabled resources
				deps, _ := c.FindModuleResources(relFQDN.String(), true)

				for _, dep := range deps {
					dependencies[dep] = true
				}
			}

			// when the dependency is a resource, depend on the resource
			if fqdn.Type != resources.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resource.Metadata().Module)

				// we ignore the error here as it may be possible that the module depends on
				// disabled resources
				dep, _ := c.FindResource(relFQDN.String())

				dependencies[dep] = true
			}
		}

		// if this resource is part of a module make it depend on that module
		if resource.Metadata().Module != "" {
			fqdnString := fmt.Sprintf("module.%s", resource.Metadata().Module)

			d, err := c.FindResource(fqdnString)
			if err != nil {
				pe := &errors.ParserError{}
				pe.Line = resource.Metadata().Line
				pe.Column = resource.Metadata().Column
				pe.Filename = resource.Metadata().File
				pe.Message = fmt.Sprintf("unable to find parent module: '%s', error: %s", fqdnString, err)
				pe.Level = errors.ParserErrorLevelError

				return nil, pe
			}

			hasDeps = true
			dependencies[d] = true
		}

		// add provider dependency for non-provider resources
		if resource.Metadata().Type != resources.TypeProvider {
			providerName := getResourceProviderName(resource)
			if providerName != "" {
				providerFQDN := fmt.Sprintf("provider.%s", providerName)
				if resource.Metadata().Module != "" {
					providerFQDN = fmt.Sprintf("module.%s.provider.%s", resource.Metadata().Module, providerName)
				}

				provider, err := c.FindResource(providerFQDN)
				if err == nil {
					hasDeps = true
					dependencies[provider] = true
				}
				// Note: We don't error if provider not found, as it might be a default/builtin provider
			}
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
func buildDestroyDAG(toDestroy []types.Resource) (*dag.AcyclicGraph, error) {
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
	destroyMap := make(map[string]types.Resource)
	for _, resource := range toDestroy {
		destroyMap[resource.Metadata().ID] = resource
	}
	
	// Add REVERSED dependencies between resources to be destroyed
	// If A depends on B, we want to destroy A before B, so we create edge A -> B
	resourcesWithDeps := make(map[types.Resource]bool)
	
	for _, resource := range toDestroy {
		// Collect all dependencies for this resource
		var allDependencies []string
		
		// Add explicit dependencies from depends_on
		allDependencies = append(allDependencies, resource.GetDependencies()...)
		
		// Add implicit dependencies from resource links (interpolations)
		allDependencies = append(allDependencies, resource.Metadata().Links...)
		
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
		
		// Check if this resource has any incoming edges from other destroy resources
		for _, otherResource := range toDestroy {
			if otherResource == resource {
				continue
			}
			
			// Check if otherResource depends on this resource
			otherDeps := append(otherResource.GetDependencies(), otherResource.Metadata().Links...)
			for _, dep := range otherDeps {
				if dep == resource.Metadata().ID {
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
		r, ok := v.(types.Resource)
		// not a resource skip, this should never happen
		if !ok {
			panic("an item has been added to the destroy graph that is not a resource")
		}
		
		// Skip the destroy root node
		if r.Metadata().Type == resources.TypeRoot {
			return nil
		}
		
		// Skip builtin resource types that don't have providers
		if r.Metadata().Type == resources.TypeVariable ||
			r.Metadata().Type == resources.TypeProvider ||
			r.Metadata().Type == resources.TypeOutput ||
			r.Metadata().Type == resources.TypeLocal ||
			r.Metadata().Type == resources.TypeModule ||
			r.Metadata().Type == resources.TypeRoot {
			
			// Fire destroy events for builtin types (always succeed with 0 time)
			resourceType := fmt.Sprintf("%s.%s", r.Metadata().Type, r.Metadata().Name)
			fireParserEvent(options, "destroy", resourceType, r.Metadata().ID, "success", 0, nil, nil)
			r.Metadata().Status = "destroyed"
			return nil
		}
		
		// Get the provider for this resource
		adapter := registry.GetProviderAdapter(r)
		if adapter == nil {
			r.Metadata().Status = "destroy_failed"
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf("no provider found for resource type %s", r.Metadata().Type)
			pe.Level = errors.ParserErrorLevelError
			return diags.Append(pe)
		}
		
		ctx := context.Background()
		resourceID := r.Metadata().ID
		resourceType := fmt.Sprintf("%s.%s", r.Metadata().Type, r.Metadata().Name)
		
		// Serialize the resource to JSON for provider call
		resourceJSON, err := json.Marshal(r)
		if err != nil {
			r.Metadata().Status = "destroy_failed"
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf("failed to serialize resource for destroy: %s", err)
			pe.Level = errors.ParserErrorLevelError
			return diags.Append(pe)
		}
		
		// Call destroy on the provider
		fireParserEvent(options, "destroy", resourceType, resourceID, "start", 0, nil, resourceJSON)
		start := time.Now()
		err = adapter.Destroy(ctx, resourceJSON, false)
		duration := time.Since(start)
		
		if err != nil {
			fireParserEvent(options, "destroy", resourceType, resourceID, "error", duration, err, resourceJSON)
			r.Metadata().Status = "destroy_failed"
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf("destroy failed: %s", err)
			pe.Level = errors.ParserErrorLevelError
			return diags.Append(pe)
		}
		
		fireParserEvent(options, "destroy", resourceType, resourceID, "success", duration, nil, resourceJSON)
		r.Metadata().Status = "destroyed"
		
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

// walkCallback creates the internal callback that is called when a node in the
// dag is visited. This callback is responsible for processing the resource and setting
// any linked values
func walkCallback(c *Config, previousState *Config, registry *PluginRegistry, options *ParserOptions) func(v dag.Vertex) (diags dag.Diagnostics) {

	return func(v dag.Vertex) (diags dag.Diagnostics) {

		r, ok := v.(types.Resource)
		// not a resource skip, this should never happen
		if !ok {
			panic("an item has been added to the graph that is not a resource")
		}

		// if this is the root module or is disabled skip or is a variable
		if r.Metadata().Type == resources.TypeRoot {
			return nil
		}

		// Handle providers specially - they have different processing requirements
		if r.Metadata().Type == resources.TypeProvider {
			// Providers need the global context with all variable resolutions
			// For providers, we only need to call the lifecycle method, no further processing
			if err := callProviderLifecycle(r, rootContext, previousState, registry, options); err != nil {
				pe := &errors.ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf("provider lifecycle error: %s", err)
				pe.Level = errors.ParserErrorLevelError
				return diags.Append(pe)
			}
			return nil
		}

		bdy, err := c.getBody(r)
		if err != nil {
			panic(fmt.Sprintf(`no body found for resource "%s"`, r.Metadata().ID))
		}

		ctx, err := c.getContext(r)
		if err != nil {
			panic("no context found for resource")
		}

		// first we need to check if the resource is disabled
		// this might be set by an interpolated value
		// if this is disabled we ignore the resource
		//
		// This expression could be a reference to another resource or it could be a
		// function or a conditional statement. We need to evaluate the expression
		// to determine if the resource should be disabled
		if attr, ok := bdy.Attributes["disabled"]; ok {
			expr, err := processExpr(attr.Expr)

			// need to handle this error
			if err != nil {
				pe := &errors.ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`unable to process disabled expression: %s`, err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}

			if len(expr) > 0 {
				// first we need to build the context for the expression
				err := setContextVariablesFromList(c, r, expr, ctx)
				if err != nil {
					return diags.Append(err)
				}

				// now we need to evaluate the expression
				var isDisabled bool
				expdiags := gohcl.DecodeExpression(attr.Expr, ctx, &isDisabled)
				if expdiags.HasErrors() {

					pe := &errors.ParserError{}
					pe.Filename = r.Metadata().File
					pe.Line = r.Metadata().Line
					pe.Column = r.Metadata().Column
					pe.Message = fmt.Sprintf(`unable to process disabled expression: %s`, expdiags.Error())
					pe.Level = errors.ParserErrorLevelError

					return diags.Append(pe)
				}

				r.SetDisabled(isDisabled)
			}
		}

		// if the resource is disabled we need to skip the resource
		if r.GetDisabled() {
			return nil
		}

		// set the context variables from the linked resources
		setContextVariablesFromList(c, r, r.Metadata().Links, ctx)

		// Process the raw resource now we have the context from the linked
		// resources
		ul := getContextLock(ctx)
		defer ul()

		// if there are defaults defined on the resource set them
		defaults.Set(r)

		diag := gohcl.DecodeBody(bdy, ctx, r)
		if diag.HasErrors() {
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf(`unable to decode body: %s`, diag.Error())
			// this error is set as warning as it is possible that the resource has
			// interpolation that is not yet resolved

			// check the error types and determine if we should set a warning or error
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

			pe.Level = level

			return diags.Append(pe)
		}

		// if the type is a module then potentially we only just found out that we should be
		// disabled

		// as an additional check, set all module resources to disabled if the module is disabled
		if r.GetDisabled() && r.Metadata().Type == resources.TypeModule {
			// find all dependent resources
			dr, err := c.FindModuleResources(r.Metadata().ID, true)
			if err != nil {
				// should not be here unless an internal error
				pe := &errors.ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf(`unable to find disabled module resources "%s", %s"`, r.Metadata().ID, err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}

			// set all the dependents to disabled
			for _, d := range dr {
				d.SetDisabled(true)
			}
		}

		// if the type is a module we need to add the variables to the
		// context
		if r.Metadata().Type == resources.TypeModule {
			mod := r.(*resources.Module)

			var mapVars map[string]cty.Value
			if att, ok := mod.Variables.(*hcl.Attribute); ok {
				val, _ := att.Expr.Value(ctx)
				mapVars = val.AsValueMap()

				for k, v := range mapVars {
					setContextVariable(mod.SubContext, k, v)
				}
			}
		}

		// if this is an output or local we need to convert the value into
		// a go type
		if r.Metadata().Type == resources.TypeOutput {
			o := r.(*resources.Output)

			if !o.CtyValue.IsNull() {
				o.Value = castVar(o.CtyValue)
			}
		}

		if r.Metadata().Type == resources.TypeLocal {
			o := r.(*resources.Local)

			if !o.CtyValue.IsNull() {
				o.Value = castVar(o.CtyValue)
			}
		}

		// if disabled was set through interpolation, the value has only been set here
		// we need to handle an additional check
		if !r.GetDisabled() {
			// Call provider lifecycle methods
			if err := callProviderLifecycle(r, ctx, previousState, registry, options); err != nil {
				pe := &errors.ParserError{}
				pe.Filename = r.Metadata().File
				pe.Line = r.Metadata().Line
				pe.Column = r.Metadata().Column
				pe.Message = fmt.Sprintf("provider lifecycle error: %s", err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}
		}

		return nil
	}
}

// callProviderLifecycle calls the appropriate provider lifecycle methods for a resource
func callProviderLifecycle(resource types.Resource, evalCtx *hcl.EvalContext, previousState *Config, registry *PluginRegistry, options *ParserOptions) error {

	// Handle provider initialization 
	if resource.Metadata().Type == resources.TypeProvider {
		return initializeProvider(resource.(*resources.Provider), evalCtx, registry, options)
	}

	// Skip other builtin resource types that don't have providers, but fire events for them
	if resource.Metadata().Type == resources.TypeVariable ||
		resource.Metadata().Type == resources.TypeOutput ||
		resource.Metadata().Type == resources.TypeLocal ||
		resource.Metadata().Type == resources.TypeModule ||
		resource.Metadata().Type == resources.TypeRoot {

		// Fire events for all builtin types (always succeed with 0 time)
		// Note: Variables are also handled in parseVariablesInFile, but this catches any that go through DAG
		resourceType := fmt.Sprintf("%s.%s", resource.Metadata().Type, resource.Metadata().Name)
		fireParserEvent(options, "create", resourceType, resource.Metadata().ID, "success", 0, nil, nil)

		return nil
	}

	// Get the provider for this resource
	adapter := registry.GetProviderAdapter(resource)
	if adapter == nil {
		// No provider found - this might be a builtin type without a provider
		return fmt.Errorf("no provider found for resource type %s", resource.Metadata().Type)
	}

	ctx := context.Background()
	resourceID := resource.Metadata().ID
	resourceType := fmt.Sprintf("%s.%s", resource.Metadata().Type, resource.Metadata().Name)

	// Serialize the current resource to JSON
	currentJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// Check if resource exists in state
	var stateResource types.Resource
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
		err := adapter.Refresh(ctx, currentJSON)
		duration := time.Since(start)
		
		if err != nil {
			fireParserEvent(options, "refresh", resourceType, resourceID, "error", duration, err, currentJSON)
			resource.Metadata().Status = "failed"
			return fmt.Errorf("refresh failed: %w", err)
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
			resource.Metadata().Status = "failed"
			return fmt.Errorf("changed check failed: %w", err)
		}
		fireParserEvent(options, "changed", resourceType, resourceID, "success", duration, nil, currentJSON)

		// 4. If changed, call Update
		if changed {
			fireParserEvent(options, "update", resourceType, resourceID, "start", 0, nil, currentJSON)
			start = time.Now()
			err := adapter.Update(ctx, currentJSON)
			duration = time.Since(start)
			
			if err != nil {
				fireParserEvent(options, "update", resourceType, resourceID, "error", duration, err, currentJSON)
				resource.Metadata().Status = "failed"
				return fmt.Errorf("update failed: %w", err)
			}
			fireParserEvent(options, "update", resourceType, resourceID, "success", duration, nil, currentJSON)
			resource.Metadata().Status = "updated"
		} else {
			// No changes needed, preserve existing status
			if stateResource.Metadata().Status != "" {
				resource.Metadata().Status = stateResource.Metadata().Status
			}
		}
	} else {
		// Resource doesn't exist in state - create it
		fireParserEvent(options, "create", resourceType, resourceID, "start", 0, nil, currentJSON)
		start := time.Now()
		err := adapter.Create(ctx, currentJSON)
		duration := time.Since(start)
		
		if err != nil {
			fireParserEvent(options, "create", resourceType, resourceID, "error", duration, err, currentJSON)
			resource.Metadata().Status = "failed"
			return fmt.Errorf("create failed: %w", err)
		}
		fireParserEvent(options, "create", resourceType, resourceID, "success", duration, nil, currentJSON)
		resource.Metadata().Status = "created"
	}

	return nil
}

// setContextVariablesFromList sets the context variables from a list of resource links
//
// for example: given the values ["module.module1.module2.resource.container.mine.id"]
// the context variable "module.module1.module2.resource.container.mine.id" will be set to the
// value defined by the resource of type container with the name mine and the attribute id
func setContextVariablesFromList(c *Config, r types.Resource, values []string, ctx *hcl.EvalContext) *errors.ParserError {
	// attempt to set the values in the resource links to the resource attribute
	// all linked values should now have been processed as the graph
	// will have handled them first
	for _, v := range values {
		fqrn, err := resources.ParseFQRN(v)
		if err != nil {
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf("error parsing resource link %s", err)
			pe.Level = errors.ParserErrorLevelError

			return pe
		}

		// get the value from the linked resource
		l, err := c.FindRelativeResource(v, r.Metadata().Module)
		if err != nil {
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf(`unable to find dependent resource "%s" %s`, v, err)
			pe.Level = errors.ParserErrorLevelError

			return pe
		}

		var ctyRes cty.Value

		// once we have found a resource convert it to a cty type and then
		// set it on the context
		switch l.Metadata().Type {
		case resources.TypeLocal:
			loc := l.(*resources.Local)
			ctyRes = loc.CtyValue
		case resources.TypeOutput:
			out := l.(*resources.Output)
			ctyRes = out.CtyValue
		default:
			ctyRes, err = convert.GoToCtyValue(l)
		}

		if err != nil {
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf(`unable to convert reference %s to context variable: %s`, v, err)
			pe.Level = errors.ParserErrorLevelError

			return pe
		}

		// remove the attributes and to get a pure resource ref
		fqrn.Attribute = ""

		err = setContextVariableFromPath(ctx, fqrn.String(), ctyRes)
		if err != nil {
			pe := &errors.ParserError{}
			pe.Filename = r.Metadata().File
			pe.Line = r.Metadata().Line
			pe.Column = r.Metadata().Column
			pe.Message = fmt.Sprintf(`unable to set context variable: %s`, err)
			pe.Level = errors.ParserErrorLevelError

			return pe
		}
	}

	return nil
}

// getResourceProviderName extracts the provider name for a resource
// Returns the explicit provider name if set, otherwise defaults to resource type
func getResourceProviderName(resource types.Resource) string {
	// First check if resource has explicit provider field using reflection
	val := reflect.ValueOf(resource)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		providerField := val.FieldByName("Provider")
		if providerField.IsValid() && providerField.Kind() == reflect.String {
			explicitProvider := providerField.String()
			if explicitProvider != "" {
				return explicitProvider
			}
		}
	}

	// Fall back to resource type as default provider name
	// Skip builtin types that don't have providers
	resourceType := resource.Metadata().Type
	if resourceType == resources.TypeVariable ||
		resourceType == resources.TypeProvider ||
		resourceType == resources.TypeOutput ||
		resourceType == resources.TypeLocal ||
		resourceType == resources.TypeModule ||
		resourceType == resources.TypeRoot {
		return ""
	}

	return resourceType
}


// initializeProvider initializes a provider during DAG execution
// This resolves provider config interpolations and registers the provider with the plugin registry
func initializeProvider(provider *resources.Provider, evalCtx *hcl.EvalContext, registry *PluginRegistry, options *ParserOptions) error {
	resourceType := fmt.Sprintf("%s.%s", provider.Metadata().Type, provider.Metadata().Name)
	resourceID := provider.Metadata().ID
	
	// Serialize provider data for events
	providerJSON, jsonErr := json.Marshal(provider)
	var data []byte
	if jsonErr == nil {
		data = providerJSON
	}
	
	// Fire start event
	fireParserEvent(options, "create", resourceType, resourceID, "start", 0, nil, data)
	start := time.Now()
	
	// Skip if already initialized
	if provider.Initialized {
		duration := time.Since(start)
		fireParserEvent(options, "create", resourceType, resourceID, "success", duration, nil, data)
		return nil
	}
	
	// Validate required fields
	if provider.Metadata().Name == "" {
		err := fmt.Errorf("provider name cannot be empty")
		fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
		return err
	}
	if provider.Source == "" {
		err := fmt.Errorf("provider source cannot be empty")
		fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
		return err
	}

	// Find plugin by source
	plugin, err := registry.findPluginBySource(provider.Source)
	if err != nil {
		err = fmt.Errorf("failed to find plugin for source '%s': %w", provider.Source, err)
		fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
		return err
	}
	if plugin == nil {
		err := fmt.Errorf("plugin for source '%s' not found", provider.Source)
		fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
		return err
	}

	// Get config type from plugin
	configType := plugin.GetConfigType()
	if configType == nil {
		err := fmt.Errorf("plugin for source '%s' does not define a configuration type", provider.Source)
		fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
		return err
	}

	// Convert hcl.Body config to concrete type if config is provided
	if provider.Config != nil {
		configBody, ok := provider.Config.(hcl.Body)
		if !ok {
			panic(fmt.Sprintf("provider config should be hcl.Body at this point, got %T", provider.Config))
		}
		
		// Create an instance of the concrete config type
		configPtr := reflect.New(configType).Interface()
		
		// Decode the config body to the concrete type using the evaluation context
		diags := gohcl.DecodeBody(configBody, evalCtx, configPtr)
		if diags.HasErrors() {
			err := fmt.Errorf("failed to decode provider config: %s", diags.Error())
			fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
			return err
		}
		
		// Store the concrete config
		provider.Config = configPtr
	}

	// Set up plugin-specific fields
	provider.ConfigType = configType
	provider.Initialized = true

	// Update provider context variable with resolved config (only if context available)
	if evalCtx != nil {
		err = updateProviderContextVariable(provider, evalCtx)
		if err != nil {
			err = fmt.Errorf("failed to update provider context variable: %w", err)
			fireParserEvent(options, "create", resourceType, resourceID, "error", time.Since(start), err, data)
			return err
		}
	}

	duration := time.Since(start)
	fireParserEvent(options, "create", resourceType, resourceID, "success", duration, nil, data)
	return nil
}

// updateProviderContextVariable updates the provider's context variable with resolved config
func updateProviderContextVariable(provider *resources.Provider, evalCtx *hcl.EvalContext) error {
	// Convert provider to cty value
	providerValue, err := convert.GoToCtyValue(provider)
	if err != nil {
		return fmt.Errorf("failed to convert provider '%s' to context value: %w", provider.Metadata().Name, err)
	}

	// If provider has resolved config, add it to the context value
	if provider.Config != nil {
		configValue, err := convert.GoToCtyValue(provider.Config)
		if err != nil {
			return fmt.Errorf("failed to convert provider config to context value: %w", err)
		}

		// Create a new object type that includes the config field
		originalAttrs := providerValue.Type().AttributeTypes()
		newAttrs := make(map[string]cty.Type)
		for k, v := range originalAttrs {
			newAttrs[k] = v
		}
		newAttrs["config"] = configValue.Type()

		// Create new object value with config included
		newValues := make(map[string]cty.Value)
		for k, _ := range originalAttrs {
			newValues[k] = providerValue.GetAttr(k)
		}
		newValues["config"] = configValue

		providerValue = cty.ObjectVal(newValues)
	}

	// Update the context variable
	return setContextVariableFromPath(evalCtx, fmt.Sprintf("provider.%s", provider.Metadata().Name), providerValue)
}
