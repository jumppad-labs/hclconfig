package hclconfig

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcl/hclsyntax"
	"github.com/hashicorp/hcl2/hclparse"
	"github.com/shipyard-run/hclconfig/lookup"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var rootContext *hcl.EvalContext

type ResourceTypeNotExistError struct {
	Type string
	File string
}

func (r ResourceTypeNotExistError) Error() string {
	return fmt.Sprintf("Resource type %s defined in file %s, does not exist. Please check the documentation for supported resources. We love PRs if you would like to create a resource of this type :)", r.Type, r.File)
}

type ParserOptions struct {
	Variables         map[string]string
	VariablesFiles    []string
	VariableEnvPrefix string
	ModuleCache       string
	Callback          ParseCallback
}

// DefaultOptions returns a ParserOptions object with the
// ModuleCache set to the default directory of $HOME/.hclconfig/cache
// if the $HOME folder can not be determined, the cache is set to the
// current folder
func DefaultOptions() *ParserOptions {
	cacheDir, err := os.UserHomeDir()
	if err != nil {
		cacheDir = "."
	}

	cacheDir = filepath.Join(".", ".hclconfig", "cache")
	os.MkdirAll(cacheDir, os.ModePerm)

	return &ParserOptions{
		ModuleCache:       cacheDir,
		VariableEnvPrefix: "HCL_VAR_",
	}
}

// Parser can parse HCL configuration files
type Parser struct {
	options         ParserOptions
	registeredTypes types.RegisteredTypes
	config          *Config
}

// NewParser creates a new parser with the given options
// if options are nil, default options are used
func NewParser(options *ParserOptions) *Parser {
	o := options
	if o == nil {
		o = DefaultOptions()
	}

	return &Parser{options: *o, registeredTypes: types.DefaultTypes()}
}

// RegisterType type registers a struct that implements Resource with the given name
// the parser uses this list to convert hcl defined resources into concrete types
func (p *Parser) RegisterType(name string, resource types.Resource) {
	p.registeredTypes[name] = resource
}

func (p *Parser) ParseFile(file string, c *Config) (*Config, error) {
	rootContext = buildContext(file)

	err := p.parseFile(rootContext, file, c, p.options.Variables, p.options.VariablesFiles)
	if err != nil {
		return nil, err
	}

	// process the files and resolve dependency
	c.process(p.options.Callback)

	// should return a copy of Config appended with new resources not a mutated version
	// of the original
	return c, nil
}

// ParseDirectory parses all resource and variable files in the given directory
// note: this method does not recurse into sub folders
func (p *Parser) ParseDirectory(dir string, c *Config) (*Config, error) {
	p.config = c
	rootContext = buildContext(dir)

	c, err := p.parseDirectory(rootContext, dir, c)
	if err != nil {
		return nil, err
	}

	// process the files and resolve dependency
	err = c.process(p.options.Callback)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// internal method
func (p *Parser) parseDirectory(ctx *hcl.EvalContext, dir string, c *Config) (*Config, error) {
	// get all files in a directory
	path, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", dir)
	}

	if !path.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf(" unable to list files in directory %s, error: %s", dir, err)
	}

	variablesFiles := p.options.VariablesFiles

	// first process vars files
	for _, f := range files {
		fn := filepath.Join(dir, f.Name())

		if !f.IsDir() {
			if strings.HasSuffix(fn, ".vars") {
				// add to the collection
				variablesFiles = append(variablesFiles, fn)
			}
		}
	}

	for _, f := range files {
		fn := filepath.Join(dir, f.Name())

		if !f.IsDir() {
			if strings.HasSuffix(fn, ".hcl") {
				err := p.parseFile(ctx, fn, c, p.options.Variables, variablesFiles)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return c, nil
}

// parseFile loads variables and resources from the given file
func (p *Parser) parseFile(
	ctx *hcl.EvalContext,
	file string,
	c *Config,
	variables map[string]string,
	variablesFile []string) error {

	// This must be done before any other process as the resources
	// might reference the variables
	err := p.parseVariablesInFile(ctx, file, c)
	if err != nil {
		return err
	}

	// override any variables from files
	for _, vf := range variablesFile {
		err := p.loadVariablesFromFile(ctx, vf)
		if err != nil {
			return err
		}
	}

	// override default values for variables from environment or variables map
	p.setVariables(ctx, variables)

	err = p.parseResourcesInFile(ctx, file, c, "", false, []string{})
	if err != nil {
		return err
	}

	return nil
}

// loadVariablesFromFile loads variable values from a file
func (p *Parser) loadVariablesFromFile(ctx *hcl.EvalContext, path string) error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseHCLFile(path)
	if diag.HasErrors() {
		return errors.New(diag.Error())
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
func (p *Parser) setVariables(ctx *hcl.EvalContext, vars map[string]string) {
	// first any vars defined as environment variables
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, p.options.VariableEnvPrefix) {
			parts := strings.Split(e, "=")

			if len(parts) == 2 {
				key := strings.Replace(parts[0], p.options.VariableEnvPrefix, "", -1)
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

// ParseVariableFile parses a config file for variables
func (p *Parser) parseVariablesInFile(ctx *hcl.EvalContext, file string, c *Config) error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	for _, b := range body.Blocks {
		switch b.Type {
		case string(types.TypeVariable):
			v := (&types.Variable{}).New(b.Labels[0]).(*types.Variable)

			err := decodeBody(ctx, file, b, v)
			if err != nil {
				return err
			}

			val, _ := v.Default.(*hcl.Attribute).Expr.Value(ctx)
			setContextVariableIfMissing(ctx, v.Name, val)
		}
	}

	return nil
}

// parseResourcesInFile parses a hcl file and adds any found resources to the config
func (p *Parser) parseResourcesInFile(ctx *hcl.EvalContext, file string, c *Config, moduleName string, disabled bool, dependsOn []string) error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	for _, b := range body.Blocks {
		// check the resource has a name
		if len(b.Labels) == 0 {
			return fmt.Errorf(
				"error in file '%s': resource '%s' has no name, please specify resources using the syntax 'resource_type \"name\" {}'",
				file,
				b.Type,
			)
		}

		name := b.Labels[0]

		// create the registered type if not a variable or output
		// variables and outputs are processed in a separate run
		switch types.ResourceType(b.Type) {
		case types.TypeVariable:
			continue
		case types.TypeModule:
			err := p.parseModule(ctx, c, name, file, b, moduleName, dependsOn)
			if err != nil {
				return fmt.Errorf("unable to process module: %s", err)
			}
		default:
			err := p.parseResource(ctx, c, name, file, b, moduleName, dependsOn, disabled)
			if err != nil {
				return fmt.Errorf("unable to process resource: %s", err)
			}
		}
	}

	return nil
}

func (p *Parser) parseModule(ctx *hcl.EvalContext, c *Config, name, file string, b *hclsyntax.Block, moduleName string, dependsOn []string) error {
	rt, _ := types.DefaultTypes().CreateResource(string(types.TypeModule), name)

	rt.Info().Module = moduleName
	rt.Info().DependsOn = dependsOn

	err := decodeBody(ctx, file, b, rt)
	if err != nil {
		return fmt.Errorf("error creating resource '%s' in file %s", b.Type, err)
	}

	// we need to fetch the source so that we can process the child resources
	// "source" is the attribute but we need to read this manually
	src, diags := b.Body.Attributes["source"].Expr.Value(ctx)
	if diags.HasErrors() {
		return fmt.Errorf("unable to read source from module: %s", diags.Error())
	}

	// src could be a github module or a realative folder
	// first check if it is a folder, we need to make it absolute relative to the current file
	dir := path.Dir(file)
	moduleSrc := path.Join(dir, src.AsString())
	fi, err := os.Stat(moduleSrc)
	if err != nil || !fi.IsDir() {
		// is not a directory fetch from source using go getter
		return fmt.Errorf("Go getter Not implemented, please use local module source")
	}

	// create a new config and add the resources later
	moduleConfig := NewConfig()

	// modules should have their own context so that variables are not globally scoped
	subContext := buildContext(moduleSrc)

	_, err = p.parseDirectory(subContext, moduleSrc, moduleConfig)
	if err != nil {
		return fmt.Errorf("unable to parse module directory: %s, error: %s", src.AsString(), err)
	}

	rt.(*types.Module).SubContext = subContext

	// add the module
	c.AddResource(rt)

	// we need to add the module name to all the returned resources
	for _, r := range moduleConfig.Resources {
		r.Info().Module = fmt.Sprintf("%s.%s", name, r.Info().Module)
		r.Info().Module = strings.TrimSuffix(r.Info().Module, ".")
		c.AddResource(r)
	}

	return nil
}

func (p *Parser) parseResource(ctx *hcl.EvalContext, c *Config, name, file string, b *hclsyntax.Block, moduleName string, dependsOn []string, disabled bool) error {
	rt, err := p.registeredTypes.CreateResource(b.Type, name)
	if err != nil {
		return fmt.Errorf("error in file '%s': unable to create resource '%s' %s", file, b.Type, err)
	}

	rt.Info().Module = moduleName
	rt.Info().DependsOn = dependsOn

	err = decodeBody(ctx, file, b, rt)
	if err != nil {
		return fmt.Errorf("error creating resource '%s' in file %s", b.Type, err)
	}

	setDisabled(rt, disabled)

	err = c.AddResource(rt)
	if err != nil {
		return fmt.Errorf(
			"Unable to add resource %s.%s in file %s: %s",
			b.Type,
			b.Labels[0],
			file,
			err,
		)
	}

	return nil
}

func setContextVariableIfMissing(ctx *hcl.EvalContext, key string, value cty.Value) {
	if m, ok := ctx.Variables["var"]; ok {
		if _, ok := m.AsValueMap()[key]; ok {
			return
		}
	}

	setContextVariable(ctx, key, value)
}

func setContextVariable(ctx *hcl.EvalContext, key string, value cty.Value) {
	valMap := map[string]cty.Value{}

	// get the existing map
	if m, ok := ctx.Variables["var"]; ok {
		valMap = m.AsValueMap()
	}

	valMap[key] = value

	ctx.Variables["var"] = cty.ObjectVal(valMap)
}

// setContextVariableFromPath sets a context variable using a nested structure based
// on the given path. Will create any child maps needed to satisfy the path.
// i.e "resources.foo.bar" set to "true" would return
// ctx.Variables["resources"].AsValueMap()["foo"].AsValueMap()["bar"].True() = true
func setContextVariableFromPath(ctx *hcl.EvalContext, path string, value cty.Value) {
	ul := getContextLock(ctx)
	defer ul()

	pathParts := strings.Split(path, ".")
	ctx.Variables = setMapVariableFromPath(ctx.Variables, pathParts, value)
}

func setMapVariableFromPath(root map[string]cty.Value, path []string, value cty.Value) map[string]cty.Value {
	// it is possible for root to be nil, ensure this is set to an empty map
	if root == nil {
		root = map[string]cty.Value{}
	}

	// is this the last path so set the value and return
	if len(path) == 1 {
		root[path[0]] = value
		return root
	}

	// if not we need to create a map node if it does not exist
	// and recurse
	val, ok := root[path[0]]
	if !ok {
		// if not we need to create a map node
		// set a map and recurse
		val = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
	}

	updated := setMapVariableFromPath(val.AsValueMap(), path[1:], value)
	root[path[0]] = cty.ObjectVal(updated)

	return root
}

func buildContext(filePath string) *hcl.EvalContext {
	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{},
		Variables: map[string]cty.Value{},
	}

	valMap := map[string]cty.Value{}
	ctx.Variables["resource"] = cty.ObjectVal(valMap)

	ctx.Functions = getDefaultFunctions(filePath)

	return ctx
}

func copyContext(path string, ctx *hcl.EvalContext) *hcl.EvalContext {
	newCtx := buildContext(path)
	newCtx.Variables = ctx.Variables

	return newCtx
}

func decodeBody(ctx *hcl.EvalContext, path string, b *hclsyntax.Block, p interface{}) error {
	dr, err := getDependentResources(b, ctx, p, "")
	if err != nil {
		return err
	}

	// filter the list so that they are unique
	uniqueResources := []string{}
	for _, v := range dr {
		found := false
		for _, r := range uniqueResources {
			if r == v {
				found = true
				break
			}
		}

		if !found {
			uniqueResources = append(uniqueResources, v)
		}
	}

	// if variable process the body, everything else
	// lazy process on dag walk
	if b.Type == string(types.TypeVariable) {
		diag := gohcl.DecodeBody(b.Body, ctx, p)
		if diag.HasErrors() {
			return errors.New(diag.Error())
		}
	}

	// set the dependent resources
	res := p.(types.Resource)
	res.Info().ResouceLinks = uniqueResources
	res.Info().Context = ctx
	res.Info().Body = b.Body

	return nil
}

// Recurively checks the fields and blocks on the resource to identify links to other resources
// i.e. resource.container.network[0].name
// when a link is found it is replaced with an empty value of the correct type and the
// dependent resources are returned to be processed later
func getDependentResources(b *hclsyntax.Block, ctx *hcl.EvalContext, resource interface{}, path string) ([]string, error) {
	references := []string{}

	for _, a := range b.Body.Attributes {
		refs, err := processExpr(a.Expr)
		if err != nil {
			return nil, err
		}

		references = append(references, refs...)
	}

	// we need to keep a count of the current block so that we
	// can get this
	blockIndex := map[string]int{}
	for _, b := range b.Body.Blocks {
		if _, ok := blockIndex[b.Type]; ok {
			blockIndex[b.Type]++
		} else {
			blockIndex[b.Type] = 0
		}

		ref := fmt.Sprintf("%s.%s[%d]", path, b.Type, blockIndex[b.Type])
		ref = strings.TrimPrefix(ref, ".")
		cr, err := getDependentResources(b, ctx, resource, ref)
		if err != nil {
			return nil, err
		}

		references = append(references, cr...)
	}

	return references, nil
}

// processAttribute extracts the necessary data out of the HCL
// attribute like a function or resource parameter so we can determine
// which attributes are lazy evaluated due to dependency on another resource.
// Attributes can be nested, therefore this function needs to return an array of
// attributes
// examples:
// something = resource.mine.attr
// something = env(resource.mine.attr)
// something = "testing/${resource.mine.attr}"
// something = "testing/${env(resource.mine.attr)}"
func processExpr(expr hclsyntax.Expression) ([]string, error) {
	resources := []string{}

	switch expr.(type) {
	case *hclsyntax.TemplateExpr:
		// a template is a mix of functions, scope expressions and literals
		// we need to check each part
		for _, v := range expr.(*hclsyntax.TemplateExpr).Parts {
			res, err := processExpr(v)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	case *hclsyntax.FunctionCallExpr:
		for _, v := range expr.(*hclsyntax.FunctionCallExpr).Args {
			res, err := processExpr(v)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
		// a function can contain args that may also have an expression
	case *hclsyntax.ScopeTraversalExpr:
		ref, err := processScopeTraversal(expr.(*hclsyntax.ScopeTraversalExpr))
		if err != nil {
			return nil, err
		}

		// only add if a resource has been returned
		if ref != "" {
			resources = append(resources, ref)
		}

	case *hclsyntax.ObjectConsExpr:
		for _, v := range expr.(*hclsyntax.ObjectConsExpr).Items {
			res, err := processExpr(v.ValueExpr)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	}

	return resources, nil
}

func processScopeTraversal(expr *hclsyntax.ScopeTraversalExpr) (string, error) {
	strExpression := ""
	for i, t := range expr.Traversal {
		if i == 0 {
			strExpression += t.(hcl.TraverseRoot).Name

			// if this is not a resource reference quit
			if strExpression != "resource" && strExpression != "module" {
				return "", nil
			}
		} else {
			// does this exist in the context
			strExpression += "." + t.(hcl.TraverseAttr).Name
		}
	}

	// add to the references collection and replace with a nil value
	// we will resolve these references before processing
	return strExpression, nil
}

func addEmptyValueToContext(ctx *hcl.EvalContext, path string, resource interface{}, attr *hclsyntax.Attribute) error {
	attrRef := path + "." + attr.Name
	attrRef = strings.TrimPrefix(attrRef, ".")

	// we need to find the type of the field we will eventually deserialze into so that
	// we can replace this with a null value for now
	t := findTypeFromInterface(attrRef, resource)

	switch t {
	case "string":
		attr.Expr = &hclsyntax.LiteralValueExpr{Val: cty.StringVal(""), SrcRange: attr.Expr.Range()}
	case "int":
		attr.Expr = &hclsyntax.LiteralValueExpr{Val: cty.NumberIntVal(0), SrcRange: attr.Expr.Range()}
	case "bool":
		attr.Expr = &hclsyntax.LiteralValueExpr{Val: cty.BoolVal(false), SrcRange: attr.Expr.Range()}
	case "ptr":
		attr.Expr = &hclsyntax.LiteralValueExpr{Val: cty.DynamicVal, SrcRange: attr.Expr.Range()}
	case "[]string":
		attr.Expr = &hclsyntax.LiteralValueExpr{Val: cty.SetVal([]cty.Value{cty.StringVal("")}), SrcRange: attr.Expr.Range()}
	case "[]int":
		attr.Expr = &hclsyntax.LiteralValueExpr{Val: cty.SetVal([]cty.Value{cty.NumberIntVal(0)}), SrcRange: attr.Expr.Range()}

	default:
		return fmt.Errorf("unable to link resource as it references an unspported type %s", t)
	}

	return nil
}

// recurses throught destination object and returns the type of the field marked by path
// e.g path "volume[1].source" is string
func findTypeFromInterface(path string, s interface{}) string {
	// strip the indexes as we are doing the lookup on a empty struct
	re, _ := regexp.Compile("\\[[0-9]+\\]")
	stripped := re.ReplaceAllString(path, "")

	value := reflect.ValueOf(s).Type()
	val, found := lookup.LookupType(value, strings.Split(stripped, "."), false, []string{"hcl", "json"})

	if !found {
		return ""
	}

	return val.String()
}

// ensureAbsolute ensure that the given path is either absolute or
// if relative is converted to abasolute based on the path of the config
func ensureAbsolute(path, file string) string {
	// if the file starts with a / and we are on windows
	// we should treat this as absolute
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		return filepath.Clean(path)
	}

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	// path is relative so make absolute using the current file path as base
	file, _ = filepath.Abs(file)

	baseDir := file
	// check if the basepath is a file return its directory
	s, _ := os.Stat(file)
	if !s.IsDir() {
		baseDir = filepath.Dir(file)
	}

	fp := filepath.Join(baseDir, path)

	return filepath.Clean(fp)
}

// setDisabled sets the disabled flag on a resource when the
// parent is disabled
func setDisabled(r types.Resource, parentDisabled bool) {
	if parentDisabled {
		r.Info().Disabled = true
	}

	// when the resource is disabled set the status
	// so the engine will not create or delete it
	if r.Info().Disabled {
		r.Info().Status = "disabled"
	}
}
