package hclconfig

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

	"github.com/jumppad-labs/hclconfig/convert"
	"github.com/jumppad-labs/hclconfig/errors"
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
	root, _ := types.DefaultTypes().CreateResource(types.TypeRoot, "root")
	graph.Add(root)

	// Loop over all resources and add to graph
	for _, resource := range c.Resources {
		// ignore variables
		if resource.Metadata().Type != types.TypeVariable {
			graph.Add(resource)
		}
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		hasDeps := false

		// do nothing with variables
		if resource.Metadata().Type == types.TypeVariable {
			continue
		}

		// we might not yet know if the resource is disabled, this could be due
		// to the value being set from a variable or an interpolated value

		// if disabled ignore any dependencies
		if resource.Metadata().Disabled {
			// add all disabled resources to the root
			//fmt.Println("connect", "root", "to", resource.Metadata().ID)

			graph.Connect(dag.BasicEdge(root, resource))
			continue
		}

		// use a map to keep a unique list
		dependencies := map[types.Resource]bool{}

		// add links to dependencies
		// this is here for now as we might need to process these two lists separately
		resource.Metadata().DependsOn = append(resource.Metadata().DependsOn, resource.Metadata().ResourceLinks...)

		for _, d := range resource.Metadata().DependsOn {
			var err error
			fqdn, err := types.ParseFQRN(d)
			if err != nil {
				pe := errors.ParserError{}
				pe.Line = resource.Metadata().SourceLine
				pe.Column = resource.Metadata().SourceColumn
				pe.Filename = resource.Metadata().SourceFile
				pe.Message = fmt.Sprintf("invalid dependency: %s, error: %s", d, err)
				pe.Level = errors.ParserErrorLevelError

				return nil, pe
			}

			// when the dependency is a module, depend on all resources in the module
			if fqdn.Type == types.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resource.Metadata().Module)
				deps, err := c.FindModuleResources(relFQDN.String(), true)
				if err != nil {
					pe := errors.ParserError{}
					pe.Line = resource.Metadata().SourceLine
					pe.Column = resource.Metadata().SourceColumn
					pe.Filename = resource.Metadata().SourceFile
					pe.Message = fmt.Sprintf("unable to find resources for module: %s, error: %s", fqdn.Module, err)
					pe.Level = errors.ParserErrorLevelError

					return nil, pe
				}

				for _, dep := range deps {
					dependencies[dep] = true
				}
			}

			// when the dependency is a resource, depend on the resource
			if fqdn.Type != types.TypeModule {
				// assume that all dependencies references have been written with no
				// knowledge of their parent module. Therefore if the parent module is
				// "module1" and the reference is "module.module2.resource.container.mine.id"
				// then the reference should be modified to include the parent reference
				// "module.module1.module2.resource.container.mine.id"
				relFQDN := fqdn.AppendParentModule(resource.Metadata().Module)
				dep, err := c.FindResource(relFQDN.String())
				if err != nil {
					pe := errors.ParserError{}
					pe.Line = resource.Metadata().SourceLine
					pe.Column = resource.Metadata().SourceColumn
					pe.Filename = resource.Metadata().SourceFile
					pe.Message = fmt.Sprintf("unable to find dependent resource in module: '%s', error: '%s'", resource.Metadata().Module, err)
					pe.Level = errors.ParserErrorLevelError

					return nil, pe
				}

				dependencies[dep] = true
			}
		}

		// if this resource is part of a module make it depend on that module
		if resource.Metadata().Module != "" {
			fqdnString := fmt.Sprintf("module.%s", resource.Metadata().Module)

			d, err := c.FindResource(fqdnString)
			if err != nil {
				pe := errors.ParserError{}
				pe.Line = resource.Metadata().SourceLine
				pe.Column = resource.Metadata().SourceColumn
				pe.Filename = resource.Metadata().SourceFile
				pe.Message = fmt.Sprintf("unable to find resources parent module: '%s, error: %s", fqdnString, err)
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

// createCallback creates the internal callback that is called when a node in the
// dag is visited. This callback is responsible for processing the resource, setting
// any linked values and calling the user defined callback so that external work
// can be performed
func createCallback(c *Config, wf WalkCallback) func(v dag.Vertex) (diags dag.Diagnostics) {
	return func(v dag.Vertex) (diags dag.Diagnostics) {

		r, ok := v.(types.Resource)
		// not a resource skip, this should never happen
		if !ok {
			panic("an item has been added to the graph that is not a resource")
		}

		// if this is the root module or is disabled skip or is a variable
		if (r.Metadata().Type == types.TypeRoot) || r.Metadata().Disabled || r.Metadata().Type == types.TypeVariable {
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

		// attempt to set the values in the resource links to the resource attribute
		// all linked values should now have been processed as the graph
		// will have handled them first
		for _, v := range r.Metadata().ResourceLinks {
			fqdn, err := types.ParseFQRN(v)
			if err != nil {
				pe := errors.ParserError{}
				pe.Filename = r.Metadata().SourceFile
				pe.Line = r.Metadata().SourceLine
				pe.Column = r.Metadata().SourceColumn
				pe.Message = fmt.Sprintf("error parsing resource link %s", err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}

			// get the value from the linked resource
			l, err := c.FindRelativeResource(v, r.Metadata().Module)
			if err != nil {
				pe := errors.ParserError{}
				pe.Filename = r.Metadata().SourceFile
				pe.Line = r.Metadata().SourceLine
				pe.Column = r.Metadata().SourceColumn
				pe.Message = fmt.Sprintf(`unable to find dependent resource "%s" %s`, v, err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}

			var ctyRes cty.Value

			// once we have found a resource convert it to a cty type and then
			// set it on the context
			switch l.Metadata().Type {
			case types.TypeLocal:
				loc := l.(*types.Local)
				ctyRes = loc.CtyValue
			case types.TypeOutput:
				out := l.(*types.Output)
				ctyRes = out.CtyValue
			default:
				ctyRes, err = convert.GoToCtyValue(l)
			}

			if err != nil {
				pe := errors.ParserError{}
				pe.Filename = r.Metadata().SourceFile
				pe.Line = r.Metadata().SourceLine
				pe.Column = r.Metadata().SourceColumn
				pe.Message = fmt.Sprintf(`unable to convert reference %s to context variable: %s`, v, err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}

			// remove the attributes and to get a pure resource ref
			fqdn.Attribute = ""

			err = setContextVariableFromPath(ctx, fqdn.String(), ctyRes)
			if err != nil {
				pe := errors.ParserError{}
				pe.Filename = r.Metadata().SourceFile
				pe.Line = r.Metadata().SourceLine
				pe.Column = r.Metadata().SourceColumn
				pe.Message = fmt.Sprintf(`unable to set context variable: %s`, err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}
		}

		// Process the raw resource now we have the context from the linked
		// resources
		ul := getContextLock(ctx)
		defer ul()

		diag := gohcl.DecodeBody(bdy, ctx, r)
		if diag.HasErrors() {
			pe := errors.ParserError{}
			pe.Filename = r.Metadata().SourceFile
			pe.Line = r.Metadata().SourceLine
			pe.Column = r.Metadata().SourceColumn
			pe.Message = fmt.Sprintf(`unable to decode body: %s`, diag.Error())
			pe.Level = errors.ParserErrorLevelWarning

			return diags.Append(pe)
		}

		// if the type is a module the potentially we only just found out that we should be
		// disabled
		// as an additional check, set all module resources to disabled if the module is disabled
		if r.Metadata().Disabled && r.Metadata().Type == types.TypeModule {
			// find all dependent resources
			dr, err := c.FindModuleResources(r.Metadata().ID, true)
			if err != nil {
				// should not be here unless an internal error
				pe := errors.ParserError{}
				pe.Filename = r.Metadata().SourceFile
				pe.Line = r.Metadata().SourceLine
				pe.Column = r.Metadata().SourceColumn
				pe.Message = fmt.Sprintf(`unable to find disabled module resources "%s", %s"`, r.Metadata().ID, err)
				pe.Level = errors.ParserErrorLevelError

				return diags.Append(pe)
			}

			// set all the dependents to disabled
			for _, d := range dr {
				d.Metadata().Disabled = true
			}
		}

		// if the config implements the processable interface call the resource process method
		// and the resource is not disabled
		//
		// if disabled was set through interpolation, the value has only been set here
		// we need to handle an additional check
		if !r.Metadata().Disabled && r.Metadata().Type != types.TypeModule {

			// call the callbacks
			if wf != nil {
				err := wf(r)
				if err != nil {
					pe := errors.ParserError{}
					pe.Filename = r.Metadata().SourceFile
					pe.Line = r.Metadata().SourceLine
					pe.Column = r.Metadata().SourceColumn
					pe.Message = fmt.Sprintf(`error calling callback for resource "%s" %s`, r.Metadata().ID, err)
					pe.Level = errors.ParserErrorLevelError

					return diags.Append(pe)
				}
			}
		}

		// if the type is a module we need to add the variables to the
		// context
		if r.Metadata().Type == types.TypeModule {
			mod := r.(*types.Module)

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
		if r.Metadata().Type == types.TypeOutput {
			o := r.(*types.Output)

			if !o.CtyValue.IsNull() {
				o.Value = castVar(o.CtyValue)
			}
		}

		if r.Metadata().Type == types.TypeLocal {
			o := r.(*types.Local)

			if !o.CtyValue.IsNull() {
				o.Value = castVar(o.CtyValue)
			}
		}

		return nil
	}
}
