package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/convert"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/types"
	"github.com/zclconf/go-cty/cty"
)

func findXclFiles(paths ...string) ([]string, error) {
	var xclFiles []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if info.IsDir() {
			// Read directory contents (non-recursive)
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("error reading directory %s: %w", path, err)
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					filePath := filepath.Join(path, entry.Name())
					if strings.ToLower(filepath.Ext(filePath)) == ".xcl" {
						xclFiles = append(xclFiles, filePath)
					}
				}
			}
		} else {
			// Check if single file has .xcl extension
			if strings.ToLower(filepath.Ext(path)) == ".xcl" {
				xclFiles = append(xclFiles, path)
			}
		}
	}

	return xclFiles, nil
}

func validateResourceName(name string) error {
	if name == "resource" || name == "module" || name == "output" || name == "variable" {
		return fmt.Errorf("invalid resource name %s, resources can not use the reserved names [resource, module, output, variable]", name)
	}

	invalidChars := `^[0-9]*$`
	r, _ := regexp.Compile(invalidChars)
	if r.MatchString(name) {
		return fmt.Errorf("invalid resource name %s, resources can not be given a numeric identifier", name)
	}

	invalidChars = `[^0-9a-zA-Z_-]`
	r, _ = regexp.Compile(invalidChars)
	if r.MatchString(name) {
		return fmt.Errorf("invalid resource name %s, resources can only contain the characters 0-9 a-z A-Z _ -", name)
	}

	return nil
}

func loadVariablesFromFile(ctx *hcl.EvalContext, path string) error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseHCLFile(path)
	if diag.HasErrors() {
		de := errors.NewParserErrorFromHCLDiag(diag[0], path)
		return de
	}

	attrs, _ := f.Body.JustAttributes()
	for name, attr := range attrs {
		val, _ := attr.Expr.Value(ctx)

		setContextVariable(ctx, name, val)
	}

	return nil
}

// setVariables allow variables to be set from a collection or environment variables
// Precedence should be file, env, vars
func setVariables(ctx *hcl.EvalContext, vars map[string]string, variableEnvPrefix string) {
	// first any vars defined as environment variables
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, variableEnvPrefix) {
			parts := strings.Split(e, "=")

			if len(parts) == 2 {
				key := strings.Replace(parts[0], variableEnvPrefix, "", -1)
				setContextVariable(ctx, key, valueFromString(parts[1]))
			}
		}
	}

	// then set vars
	for k, v := range vars {
		setContextVariable(ctx, k, valueFromString(v))
	}
}

func valueFromString(v string) cty.Value {
	// attempt to parse the string value into a known type
	if val, err := strconv.ParseInt(v, 10, 0); err == nil {
		return cty.NumberIntVal(val)
	}

	if val, err := strconv.ParseBool(v); err == nil {
		return cty.BoolVal(val)
	}

	// otherwise return a string
	return cty.StringVal(v)
}

func setContextVariableIfMissing(ctx *hcl.EvalContext, key string, value cty.Value) {
	if m, ok := ctx.Variables["variable"]; ok {
		if _, ok := m.AsValueMap()[key]; ok {
			return
		}
	}

	setContextVariable(ctx, key, value)
}

func setContextVariable(ctx *hcl.EvalContext, key string, value cty.Value) {
	// Note: context_lock.go was removed - if concurrent access is needed,
	// locking should be handled by the caller
	valMap := map[string]cty.Value{}

	// get the existing map
	if m, ok := ctx.Variables["variable"]; ok {
		// Create a copy of the map to avoid modifying the original
		existingMap := m.AsValueMap()
		if existingMap != nil {
			for k, v := range existingMap {
				valMap[k] = v
			}
		}
	}

	valMap[key] = value

	ctx.Variables["variable"] = cty.ObjectVal(valMap)
}

// setContextVariablesFromList sets context variables for all resources in the values list
// This is used to populate the evaluation context with linked resource values
func setContextVariablesFromList(s ResourceProvider, r any, values []string, ctx *hcl.EvalContext) *errors.ParserError {
	for _, v := range values {
		rMeta, err := types.GetMeta(r)
		if err != nil {
			pe := errors.NewParserError(
				"",
				0,
				0,
				errors.ParserErrorLevelError,
				fmt.Sprintf("resource does not have ResourceBase embedded: %s", err),
			)
			return pe
		}

		fqrn, err := resources.ParseFQRN(v)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("error parsing resource link %s", err),
			)
			return pe
		}

		// Get the value from the linked resource
		l, err := s.FindRelativeResource(v, rMeta.Module)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("unable to find dependent resource \"%s\" %s", v, err),
			)
			return pe
		}

		var ctyRes cty.Value

		// Convert resource to cty type
		lMeta, err := types.GetMeta(l)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("linked resource does not have ResourceBase embedded: %s", err),
			)
			return pe
		}

		switch lMeta.Type {
		case resources.TypeOutput:
			out := l.(*resources.Output)
			ctyRes = out.CtyValue
		case resources.TypeVariable:
			// For variables, only set if not already in context
			// This preserves variable file/env overrides from buildContextForResource
			// while still allowing variables to be referenced before they're processed
			variable := l.(*resources.Variable)

			// Check if this variable is already in the context
			if varObj, ok := ctx.Variables["variable"]; ok && !varObj.IsNull() && varObj.Type().IsObjectType() {
				varMap := varObj.AsValueMap()
				if _, exists := varMap[lMeta.Name]; exists {
					// Variable already in context, don't overwrite it
					continue
				}
			}

			// Variable not in context yet, use Default value
			ctyRes = variable.Default
		default:
			ctyRes, err = convert.GoToCtyValue(l)
		}

		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("unable to convert reference %s to context variable: %s", v, err),
			)
			return pe
		}

		// Remove the attributes to get a pure resource ref
		fqrn.Attribute = ""

		err = setContextVariableFromPath(ctx, fqrn.String(), ctyRes)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				r,
				errors.ParserErrorLevelError,
				fmt.Sprintf("unable to set context variable: %s", err),
			)
			return pe
		}
	}

	return nil
}

// setContextVariableFromPath sets a context variable using a nested structure based
// on the given path. Will create any child maps needed to satisfy the path.
// i.e "resources.foo.bar" set to "true" would return
// ctx.Variables["resources"].AsValueMap()["foo"].AsValueMap()["bar"].True() = true
func setContextVariableFromPath(ctx *hcl.EvalContext, path string, value cty.Value) error {
	// Note: context_lock.go was removed - if concurrent access is needed,
	// locking should be handled by the caller

	pathParts := strings.Split(path, ".")

	var err error
	ctx.Variables, err = setMapVariableFromPath(ctx.Variables, pathParts, value)

	return err
}

func setMapVariableFromPath(root map[string]cty.Value, path []string, value cty.Value) (map[string]cty.Value, error) {
	// it is possible for root to be nil, ensure this is set to an empty map
	if root == nil {
		root = map[string]cty.Value{}
	}

	// gets the name and the index from the path
	name, index, rPath, err := getNameAndIndex(path)
	if err != nil {
		return nil, err
	}

	// do we have a node at this path if not we need to create if it
	// nodes can either be a map or a list of maps
	val, ok := root[name]
	if !ok {
		if index >= 0 {
			// create a list with the correct length
			vals := make([]cty.Value, index+1)

			val = cty.ListVal(vals)
		} else {
			// create a map nodej
			val = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
		}
	}

	if index >= 0 {
		// if we have an index we need to set the list variable for the map at that
		// index and then recursively set the other elements in the map
		updated, err := setListVariableFromPath(val.AsValueSlice(), rPath, index, value)
		if err != nil {
			return nil, err
		}

		root[name] = cty.ListVal(updated)
	} else {
		// check if the value is a list, it is possible that the user is
		// trying to incorrectly access a list type using a string parameter
		// if we do not check this it will panic
		//if val.Type().IsTupleType() || val.Type().IsListType() {
		//	err := fmt.Errorf(`the parameter is a list of items, you can not use the string index "%s" to access items, please use numeric indexes`, name)
		//	return nil, err
		//}

		// if this is the end of the line set the value and return
		if len(rPath) == 0 {
			root[name] = value
			return root, nil
		}

		// we are setting a map, recurse
		updated, err := setMapVariableFromPath(val.AsValueMap(), rPath, value)
		if err != nil {
			return nil, err
		}

		root[name] = cty.ObjectVal(updated)
	}

	return root, nil
}

func setListVariableFromPath(root []cty.Value, path []string, index int, value cty.Value) ([]cty.Value, error) {
	// we have a node but do we need to expand it in size?
	if index >= len(root) {
		root = append(root, make([]cty.Value, index+1-len(root))...)
	}

	var setVal cty.Value
	if len(path) > 0 {

		val := root[index]
		if val.IsNull() {
			val = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
		}

		updated, err := setMapVariableFromPath(val.AsValueMap(), path, value)
		if err != nil {
			return nil, err
		}

		setVal = cty.ObjectVal(updated)
	} else {
		setVal = value
	}

	// check the type of the collection, if trying to set a type that is inconsistent
	// from the other types in the collection, return an error
	if len(root) > 0 {
		if root[0].Type() != cty.NilType && root[0].Type().FriendlyName() != setVal.Type().FriendlyName() {
			return nil, fmt.Errorf("lists must contain similar types, you have tried to set a %s, to a list of type %s", value.Type().FriendlyName(), root[0].Type().FriendlyName())
		}
	}

	root[index] = setVal

	// build a unique list of keys and types, if the
	// node contains a list of maps
	ul := map[string]cty.Type{}
	for _, m := range root {
		if m.Type().IsObjectType() || m.Type().IsMapType() {
			for k, v := range m.AsValueMap() {
				ul[k] = v.Type()
			}
		}
	}

	if len(ul) == 0 {
		return root, nil
	}

	// we need to normalize the map collection as cty does not allow inconsistent map keys
	for k, v := range ul {
		for i, m := range root {
			if m.IsNull() {
				m = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
			}

			if _, ok := m.AsValueMap()[k]; !ok {
				val := m.AsValueMap()
				val[k] = cty.NullVal(v)
				root[i] = cty.ObjectVal(val)
			}
		}
	}

	return root, nil
}

// gets the name of the path and the index
// if path[0] == foo     and path[1] = bar[0] returns foo, -1, nil
// if path[0] == bar[0]  and path[1] = biz    returns bar, 0, nil
// if path[0] == foo     and path[1] = 0 returns foo, 0, nil
// if path[0] == foo     and path[1] = bar returns foo, -1, nil
// if path[0] == foo     and path[1] = nil returns foo, -1, nil
func getNameAndIndex(path []string) (name string, index int, remainingPath []string, err error) {
	index = -1

	// is the path an array with parenthesis
	rg, _ := regexp.Compile(`(.*)\[(.+)\]`)
	if sm := rg.FindStringSubmatch(path[0]); len(sm) == 3 {
		name = sm[1]

		var convErr error
		index, convErr = strconv.Atoi(sm[2])
		if convErr != nil {
			return "", -1, nil, fmt.Errorf("index %s is not a number", sm[2])
		}

		return name, index, path[1:], nil
	}

	// is the path a number using the . notation for an index
	if len(path) > 1 {
		index, convErr := strconv.Atoi(path[1])
		if convErr == nil {
			return path[0], index, path[2:], nil
		}
	}

	// normal path item
	return path[0], -1, path[1:], nil
}

func getResourceDependencies(rp ResourceProvider, resource any, resourceMeta *types.Meta) (map[any]bool, error) {
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
			deps, _ := rp.FindModuleResources(relFQDN.String(), true)

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
			dep, _ := rp.FindResource(relFQDN.String())

			dependencies[dep] = true
		}
	}

	// if this resource is part of a module make it depend on that module
	if resourceMeta.Module != "" {
		fqdnString := fmt.Sprintf("module.%s", resourceMeta.Module)

		d, err := rp.FindResource(fqdnString)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				resource,
				errors.ParserErrorLevelError,
				fmt.Sprintf("unable to find parent module: '%s', error: %s", fqdnString, err),
			)
			return nil, pe
		}

		dependencies[d] = true
	}

	return dependencies, nil
}

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
