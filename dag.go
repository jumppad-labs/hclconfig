package hclconfig

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/creasty/defaults"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

	"github.com/instruqt/hclconfig/convert"
	"github.com/instruqt/hclconfig/errors"
	"github.com/instruqt/hclconfig/resources"
	"github.com/instruqt/hclconfig/types"
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
				return nil, createParserError(resource, fmt.Sprintf("invalid dependency '%s': %s", d, err))
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
				return nil, createParserError(resource,
					fmt.Sprintf("unable to find parent module: '%s', error: %s", fqdnString, err))
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

		// ensure that the resource is written to by other processes
		l := getResourceLock(r)
		defer l()

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

		// validate the resource links
		if len(r.Metadata().Links) > 0 {
			err := validateLinkedResources(c, r, r.Metadata().Links)
			if err != nil {
				return diags.Append(createParserError(
					r,
					fmt.Sprintf(`resource contains invalid interpolated values: %s`, err),
				))
			}
		}

		// if the resource is disabled we need to skip setting disabled again
		// otherwise we could revert the disabled state set by a module
		if r.GetDisabled() {
			return nil
		}

		// first we need to check if the resource is disabled
		// this might be set by an interpolated value
		// if this is disabled we ignore the resource
		//
		// This expression could be a reference to another resource or it could be a
		// function or a conditional statement. We need to evaluate the expression
		// to determine if the resource should be disabled
		if attr, ok := bdy.Attributes["disabled"]; ok {
			resources, err := processExpr(attr.Expr)

			// need to handle this error
			if err != nil {
				return diags.Append(createParserError(
					r,
					fmt.Sprintf(`unable to process disabled expression: %s`, err)),
				)
			}

			var isDisabled bool
			if len(resources) > 0 {

				// first we need to build the context for the expression
				withContextLock(ctx, func() {
					err := setContextVariablesFromList(c, r, resources, ctx)
					if err != nil {
						diags = diags.Append(err)
					}
				})

				if diags.HasErrors() {
					return diags
				}

				// now we need to evaluate the expression
				expdiags := hcl.Diagnostics{}
				withContextLock(ctx, func() {
					expdiags = gohcl.DecodeExpression(attr.Expr, ctx, &isDisabled)
				})

				if expdiags.HasErrors() {
					return diags.Append(createParserError(
						r,
						fmt.Sprintf(`unable to process disabled expression: %s`, expdiags.Error())),
					)
				}

				r.SetDisabled(isDisabled)
			}
		}

		// set the context variables from the linked resources
		withContextLock(ctx, func() {
			err := setContextVariablesFromList(c, r, r.Metadata().Links, ctx)

			if err != nil {
				diags = diags.Append(err)
			}
		})

		if diags.HasErrors() {
			return diags
		}

		// if there are defaults defined on the resource set them
		defaults.Set(r)

		// process the raw resource now we have the context from the linked
		// resources
		decodeDiags := hcl.Diagnostics{}
		withContextLock(ctx, func() {
			decodeDiags = gohcl.DecodeBody(bdy, ctx, r)
		})

		if decodeDiags.HasErrors() {
			// this error is set as warning as it is possible that the resource has
			// interpolation that is not yet resolved.
			// depending on the erro type we may later convert this to an error if it is a syntax error
			parserErr := createParserWarning(r, fmt.Sprintf(`unable to decode body: %s`, decodeDiags.Error()))

			// check the error types and determine if we should set a warning or error
			for _, e := range decodeDiags.Errs() {
				err, ok := e.(*hcl.Diagnostic)
				if !ok {
					continue
				}

				errorSummaries := []string{
					"Error in function call",
					"Call to unknown function",
					"Unknown variable",
					"Invalid expanding argument value",
					"Not enough function arguments",
					"Too many function arguments",
					"Invalid function argument",
					"Inconsistent conditional result types",
					"Null condition",
					"Incorrect condition type",
					"Null value as key",
					"Incorrect key type",
					"Ambiguous attribute key",
					"Iteration over null value",
					"Iteration over non-iterable value",
					"Condition is null",
					"Invalid 'for' condition",
					"Invalid object key",
					"Duplicate object key",
					"Splat of null value",
					"Invalid nested splat expressions",
					"Function calls not allowed",
					"Unsupported argument",
				}

				if slices.Contains(errorSummaries, err.Summary) {
					parserErr.Level = errors.ParserErrorLevelError
					return diags.Append(parserErr)
				}
			}
		}

		// if the type is a module we need to add the variables to the
		// context
		if r.Metadata().Type == resources.TypeModule {

			// if it is disabled we need to set all the
			// resources in the module to disabled
			if r.GetDisabled() {
				// find all dependent resources
				dr, err := c.FindModuleResources(r.Metadata().ID, true)
				if err != nil {
					return diags.Append(createParserError(
						r,
						fmt.Sprintf(`unable to find disabled module resources "%s", %s"`, r.Metadata().ID, err),
					))
				}

				// set all the dependents to disabled
				for _, d := range dr {
					d.SetDisabled(true)
				}

				// if the modules is disabled we can exit here
				return nil
			}

			// now set the context variables from the modules variables
			mod := r.(*resources.Module)

			withContextLock(ctx, func() {
				var mapVars map[string]cty.Value
				if att, ok := mod.Variables.(*hcl.Attribute); ok {
					val, _ := att.Expr.Value(ctx)
					mapVars = val.AsValueMap()

					for k, v := range mapVars {
						setContextVariable(mod.SubContext, k, v)
					}
				}
			})
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

		// if the config implements the processable interface call the resource process method
		// and the resource is not disabled
		//
		// if disabled was set through interpolation, the value has only been set here
		// we need to handle an additional check
		// call the callbacks
		if wf != nil {
			err := wf(r)
			if err != nil {
				return diags.Append(createParserError(
					r,
					fmt.Sprintf(`unable to create resource "%s": %s`, r.Metadata().ID, err),
				))
			}
		}

		return nil
	}
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
	for _, value := range values {
		// get the value from the linked resource
		l, err := c.FindRelativeResource(value, r.Metadata().Module)
		if err != nil {
			return createParserError(
				r,
				fmt.Sprintf("unable to find dependent resource '%s': %s", value, err))
		}

		// can we set the context variables here?
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
			return createParserError(
				r,
				fmt.Sprintf(`unable to convert reference %s to context variable: %s`, value, err))
		}

		// remove the attributes and to get a pure resource ref
		// validate the name of the resource
		fqrn, err := resources.ParseFQRN(value)
		if err != nil {
			return createParserError(r, fmt.Sprintf("error parsing resource link %s", err))
		}

		fqrn.Attribute = ""

		err = setContextVariableFromPath(ctx, fqrn.String(), ctyRes)
		if err != nil {
			return createParserError(r, fmt.Sprintf(`unable to set context variable: %s`, err))
		}
	}

	return nil
}

// validateLinkedResources validates the linked resources in a resource
// linked resources are extracted from the interpolated values in the resource
// and are expected to be in the format of "module.module1.module2.resource.container.mine.id"

// this function will check if the resource exists and if the attribute exists
// it will also check if the attribute is a valid attribute of the resource
func validateLinkedResources(c *Config, r types.Resource, values []string) error {
	for _, value := range values {
		fqrn, err := resources.ParseFQRN(value)
		if err != nil {
			return createParserError(r, fmt.Sprintf("error parsing resource link %s", err))
		}

		// get the value from the linked resource
		l, err := c.FindRelativeResource(value, r.Metadata().Module)
		if err != nil {
			return createParserError(
				r,
				fmt.Sprintf("unable to find dependent resource '%s': %s", value, err))
		}

		attr := fqrn.Attribute
		if fqrn.Type == "output" {
			if attr == "" {
				attr = "value"
			} else {
				attr = "value." + attr
			}
		}

		// if we have additional properties, check if the object has those
		if attr != "" {
			properties := strings.Split(attr, ".")

			// check if we have a bracket on the first property
			// cut it off the property and insert it into properties after the current property
			flattened := []string{}
			for _, property := range properties {
				regex := regexp.MustCompile(`(?P<property>[a-zA-Z0-9_\-]*)(?:\[["']?(?P<key>[a-zA-Z0-9)_\-]*)["']?\])?`)
				matches := regex.FindStringSubmatch(property)

				parts := make(map[string]string)
				for i, name := range regex.SubexpNames() {
					if i != 0 && name != "" {
						parts[name] = matches[i]
					}
				}

				flattened = append(flattened, parts["property"])
				if parts["key"] != "" {
					flattened = append(flattened, parts["key"])
				}

			}

			v := reflect.ValueOf(l)
			t := reflect.TypeOf(l)

			err = validateAttribute(v, t, flattened)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// validateAttribute checks if the attribute exists in the resource,
// this is to check if the user has created invalid references to attributes
// and to provide better error messages
func validateAttribute(v reflect.Value, t reflect.Type, properties []string) error {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	// Do we have to handle nested maps and maps in slices?
	switch t.Kind() {
	case reflect.Struct:
		// if we encounter a cty.Value it can be anything, so we have to assume its alright.
		if t.String() == "cty.Value" {
			return nil
		}

		// handle embedded ResourceBase
		if properties[0] == "meta" {
			r, found := t.FieldByName("ResourceBase")
			if !found {
				return fmt.Errorf(`unable to find dependent attribute "%s"`, properties[0])
			}

			m, found := r.Type.FieldByName("Meta")
			if !found {
				return fmt.Errorf(`unable to find dependent attribute "%s"`, properties[0])
			}

			// get the corresponding values to pass on
			rv := v.FieldByName("ResourceBase")
			mv := rv.FieldByName("Meta")

			return validateAttribute(mv, m.Type, properties[1:])
		}

		if properties[0] == "disabled" {
			_, found := t.FieldByName("ResourceBase")
			if !found {
				return fmt.Errorf(`unable to find dependent attribute "%s"`, properties[0])
			}

			return nil
		}

		for index := range t.NumField() {
			f := t.Field(index)

			// compare the property with hcl tags on the object
			tag := f.Tag.Get("hcl")
			if strings.Contains(tag, properties[0]) {
				// if there are no further properties, we are done.
				if len(properties) == 1 {
					return nil
				}

				fv := v.FieldByName(f.Name)

				// TODO: check if we can actually use this case, because all computed values would be nil initially
				//
				// if we have a nil value, its nested attributes can not be resolved
				// unlike an interface or cty.Value, we can be sure that the value should be known
				if fv.Type().Kind() == reflect.Ptr {
					if fv.IsNil() {
						return fmt.Errorf(`dependent attribute is not set: "%s"`, properties[0])
					}
				}

				return validateAttribute(fv, f.Type, properties[1:])
			}
		}

	case reflect.Slice:
		nt := t.Elem()

		// try to parse the index, if it fails its not a valid index
		i, err := strconv.ParseInt(properties[0], 10, 32)
		if err != nil {
			return fmt.Errorf(`invalid list index: "%s"`, properties[0])
		}

		// check that the index is not greater than the length of the slice
		if int(i) >= v.Len() {
			return fmt.Errorf(`list does not contain index: "%s"`, properties[0])
		}

		// set the nested value to the slice element
		nv := v.Index(int(i))

		// if we only have an index, we are done
		if len(properties) == 1 {
			return nil
		}

		// ignore the next property (0), because it is an index and we dont care about it
		return validateAttribute(nv, nt, properties[1:])

	case reflect.Map:
		nt := t.Elem()

		// check that the referred key exists
		var nv reflect.Value
		var keyFound bool

		keys := v.MapKeys()
		for _, key := range keys {
			if key.String() == properties[0] {
				keyFound = true
				nv = v.MapIndex(key)
			}
		}

		if !keyFound {
			return fmt.Errorf(`map does not contain key: "%s"`, properties[0])
		}

		// if there are no further properties, we are done.
		if len(properties) == 1 {
			return nil
		}

		// if we found the key, see if we can traverse into the nested value
		return validateAttribute(nv, nt, properties[1:])

	// since an interface can be anything, so we have to assume its alright.
	case reflect.Interface:
		return nil
	}

	return fmt.Errorf(`unable to find dependent attribute: "%s"`, properties[0])
}

func createParserError(r types.Resource, msg string) *errors.ParserError {
	pe := &errors.ParserError{}
	pe.Filename = r.Metadata().File
	pe.Line = r.Metadata().Line
	pe.Column = r.Metadata().Column
	pe.Message = msg
	pe.Level = errors.ParserErrorLevelError

	return pe
}

func createParserWarning(r types.Resource, msg string) *errors.ParserError {
	pe := &errors.ParserError{}
	pe.Filename = r.Metadata().File
	pe.Line = r.Metadata().Line
	pe.Column = r.Metadata().Column
	pe.Message = msg
	pe.Level = errors.ParserErrorLevelWarning

	return pe
}
