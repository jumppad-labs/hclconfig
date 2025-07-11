package hclconfig

import (
	"context"
	"encoding/json"
	"fmt"

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
		// ignore variables
		if resource.Metadata().Type != resources.TypeVariable {
			graph.Add(resource)
		}
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		// do nothing with variables
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

// walkCallback creates the internal callback that is called when a node in the
// dag is visited. This callback is responsible for processing the resource and setting
// any linked values
func walkCallback(c *Config, previousState *Config, registry *PluginRegistry) func(v dag.Vertex) (diags dag.Diagnostics) {

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
			if err := callProviderLifecycle(r, previousState, registry); err != nil {
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
func callProviderLifecycle(resource types.Resource, previousState *Config, registry *PluginRegistry) error {

	// Skip builtin resource types that don't have providers
	if resource.Metadata().Type == resources.TypeVariable ||
		resource.Metadata().Type == resources.TypeOutput ||
		resource.Metadata().Type == resources.TypeLocal ||
		resource.Metadata().Type == resources.TypeModule ||
		resource.Metadata().Type == resources.TypeRoot {
		return nil
	}

	// Get the provider for this resource
	adapter := registry.GetProvider(resource)
	if adapter == nil {
		// No provider found - this might be a builtin type without a provider
		return fmt.Errorf("no provider found for resource type %s", resource.Metadata().Type)
	}

	ctx := context.Background()
	resourceID := resource.Metadata().ID

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
		if err := adapter.Refresh(ctx, currentJSON); err != nil {
			resource.Metadata().Status = "failed"
			return fmt.Errorf("refresh failed: %w", err)
		}

		// 2. Serialize state resource for comparison
		stateJSON, err := json.Marshal(stateResource)
		if err != nil {
			return fmt.Errorf("failed to serialize state resource: %w", err)
		}

		// 3. Check if resource has changed
		changed, err := adapter.Changed(ctx, stateJSON, currentJSON)
		if err != nil {
			resource.Metadata().Status = "failed"
			return fmt.Errorf("changed check failed: %w", err)
		}

		// 4. If changed, call Update
		if changed {
			if err := adapter.Update(ctx, currentJSON); err != nil {
				resource.Metadata().Status = "failed"
				return fmt.Errorf("update failed: %w", err)
			}
			resource.Metadata().Status = "updated"
		} else {
			// No changes needed, preserve existing status
			if stateResource.Metadata().Status != "" {
				resource.Metadata().Status = stateResource.Metadata().Status
			}
		}
	} else {
		// Resource doesn't exist in state - create it
		if err := adapter.Create(ctx, currentJSON); err != nil {
			resource.Metadata().Status = "failed"
			return fmt.Errorf("create failed: %w", err)
		}
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
