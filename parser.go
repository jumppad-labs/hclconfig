package hclconfig

import (
	"encoding/json"
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
	"github.com/mitchellh/go-wordwrap"
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

// ParserError is a detailed error that is returned from the parser
type ParserError struct {
	Filename string
	Line     int
	Column   int
	Details  string
	Message  string
}

// Error pretty prints the error message as a string
func (p ParserError) Error() string {
	err := strings.Builder{}
	err.WriteString("Error:\n")

	errLines := strings.Split(wordwrap.WrapString(p.Message, 80), "\n")
	for _, l := range errLines {
		err.WriteString("  " + l + "\n")
	}

	err.WriteString("\n")

	err.WriteString("  " + fmt.Sprintf("%s:%d,%d\n", p.Filename, p.Line, p.Column))
	// process the file
	file, _ := ioutil.ReadFile(wordwrap.WrapString(p.Filename, 80))

	lines := strings.Split(string(file), "\n")

	startLine := p.Line - 3
	if startLine < 0 {
		startLine = 0
	}

	endLine := p.Line + 2
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	for i := startLine; i < endLine; i++ {
		codeline := wordwrap.WrapString(lines[i], 70)
		codelines := strings.Split(codeline, "\n")

		if i == p.Line-1 {
			err.WriteString(fmt.Sprintf("\033[1m  %5d | %s\033[0m\n", i+1, codelines[0]))
		} else {
			err.WriteString(fmt.Sprintf("\033[2m  %5d | %s\033[0m\n", i+1, codelines[0]))
		}

		for _, l := range codelines[1:] {
			if i == p.Line-1 {
				err.WriteString(fmt.Sprintf("\033[1m        : %s\033[0m\n", l))
			} else {
				err.WriteString(fmt.Sprintf("\033[2m        : %s\033[0m\n", l))
			}
		}
	}

	return err.String()
}

type ParserOptions struct {
	// list of default variable values to add to the parser
	Variables map[string]string
	// list of variable files to be read by the parser
	VariablesFiles []string
	// environment variable prefix
	VariableEnvPrefix string
	// location of any downloaded modules
	ModuleCache string
	// callback executed when the parser reads a resource stanza, callbacks are
	// executed based on a directed acyclic graph. If resource 'a' references
	// a property defined in resource 'b', i.e 'resource.a.myproperty' then the
	// callback for resource 'b' will be executed before resource 'a'. This allows
	// you to set the dependent properties of resource 'b' before resource 'a'
	// consumes them.
	ParseCallback ProcessCallback
}

// DefaultOptions returns a ParserOptions object with the
// ModuleCache set to the default directory of $HOME/.hclconfig/cache
// if the $HOME folder can not be determined, the cache is set to the
// current folder
// VariableEnvPrefix is set to 'HCL_VAR_', should a variable be defined
// called 'foo' setting the environment variable 'HCL_VAR_foo' will override
// any default value
func DefaultOptions() *ParserOptions {
	cacheDir, err := os.UserHomeDir()
	if err != nil {
		cacheDir = "."
	}

	cacheDir = filepath.Join(cacheDir, ".hclconfig", "cache")
	os.MkdirAll(cacheDir, os.ModePerm)

	return &ParserOptions{
		ModuleCache:       cacheDir,
		VariableEnvPrefix: "HCL_VAR_",
	}
}

// Parser can parse HCL configuration files
type Parser struct {
	options             ParserOptions
	registeredTypes     types.RegisteredTypes
	registeredFunctions map[string]function.Function
}

// NewParser creates a new parser with the given options
// if options are nil, default options are used
func NewParser(options *ParserOptions) *Parser {
	o := options
	if o == nil {
		o = DefaultOptions()
	}

	return &Parser{options: *o, registeredTypes: types.DefaultTypes(), registeredFunctions: map[string]function.Function{}}
}

// RegisterType type registers a struct that implements Resource with the given name
// the parser uses this list to convert hcl defined resources into concrete types
func (p *Parser) RegisterType(name string, resource types.Resource) {
	p.registeredTypes[name] = resource
}

// RegisterFunction type registers a custom interpolation function
// with the given name
// the parser uses this list to convert hcl defined resources into concrete types
func (p *Parser) RegisterFunction(name string, f interface{}) error {
	ctyFunc, err := createCtyFunctionFromGoFunc(f)
	if err != nil {
		return nil
	}

	p.registeredFunctions[name] = ctyFunc

	return nil
}

func (p *Parser) ParseFile(file string) (*Config, error) {
	c := NewConfig()
	rootContext = buildContext(file, p.registeredFunctions)

	err := p.parseFile(rootContext, file, c, p.options.Variables, p.options.VariablesFiles)
	if err != nil {
		return nil, err
	}

	// process the files and resolve dependency
	return c, c.process(c.createCallback(p.options.ParseCallback), false)
}

// ParseDirectory parses all resource and variable files in the given directory
// note: this method does not recurse into sub folders
func (p *Parser) ParseDirectory(dir string) (*Config, error) {
	c := NewConfig()
	rootContext = buildContext(dir, p.registeredFunctions)

	err := p.parseDirectory(rootContext, dir, c)
	if err != nil {
		return nil, err
	}

	// process the files and resolve dependency
	return c, c.process(c.createCallback(p.options.ParseCallback), false)
}

// internal method
func (p *Parser) parseDirectory(ctx *hcl.EvalContext, dir string, c *Config) error {

	// get all files in a directory
	path, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", dir)
	}

	if !path.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("unable to list files in directory %s, error: %s", dir, err)
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
					return err
				}
			}
		}
	}

	return nil
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
		case types.TypeVariable:
			r, _ := p.registeredTypes.CreateResource(types.TypeVariable, b.Labels[0])
			v := r.(*types.Variable)

			err := decodeBody(ctx, file, b, v)
			if err != nil {
				return err
			}

			// add the variable to the context
			c.AppendResource(v)

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
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf("resource '%s' has no name, please specify resources using the syntax 'resource_type \"name\" {}'", b.Type)

			return de
		}

		// create the registered type if not a variable or output
		// variables and outputs are processed in a separate run
		switch b.Type {
		case types.TypeVariable:
			continue
		case types.TypeModule:
			err := p.parseModule(ctx, c, file, b, moduleName, dependsOn)
			if err != nil {
				return err
			}
		case types.TypeOutput:
			fallthrough
		case types.TypeResource:
			err := p.parseResource(ctx, c, file, b, moduleName, dependsOn, disabled)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unable to process stanza '%s' in file %s at %d,%d , only 'variable', 'resource', 'module', and 'output' are valid stanza blocks", b.Type, file, b.Range().Start.Line, b.Range().Start.Column)
		}
	}

	return nil
}

func setDisabled(ctx *hcl.EvalContext, r types.Resource, b *hclsyntax.Body, parentDisabled bool) error {
	if b == nil {
		return nil
	}

	if parentDisabled {
		r.Metadata().Disabled = true
		return nil
	}

	if attr, ok := b.Attributes["disabled"]; ok {

		disabled, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return fmt.Errorf("unable to read source from module: %s", diags.Error())
		}

		r.Metadata().Disabled = disabled.True()
	}

	return nil
}

func setDependsOn(ctx *hcl.EvalContext, r types.Resource, b *hclsyntax.Body, dependsOn []string) error {
	r.Metadata().DependsOn = dependsOn

	if attr, ok := b.Attributes["depends_on"]; ok {
		dependsOnVal, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return fmt.Errorf("unable to read depends_on attribute: %s", diags.Error())
		}

		// depends on is a slice of string
		dependsOnSlice := dependsOnVal.AsValueSlice()
		for _, d := range dependsOnSlice {
			_, err := types.ParseFQRN(d.AsString())
			if err != nil {
				return fmt.Errorf("invalid dependency %s, %s", d.AsString(), err)
			}

			r.Metadata().DependsOn = append(r.Metadata().DependsOn, d.AsString())
		}
	}

	return nil
}

func (p *Parser) parseModule(ctx *hcl.EvalContext, c *Config, file string, b *hclsyntax.Block, moduleName string, dependsOn []string) error {
	// check the module has a name
	if len(b.Labels) != 1 {
		return fmt.Errorf(`error in file %s at position %d,%d, invalid syntax for 'module' stanza, modules should be formatted 'module "name" {}`, file, b.Range().Start.Line, b.TypeRange.Start.Column)
	}

	name := b.Labels[0]
	if name == "resource" || name == "module" || name == "output" {
		de := ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`invalid resource name "%s", resource, name, and output are reserved resource names`, name)

		return de
	}

	rt, _ := types.DefaultTypes().CreateResource(string(types.TypeModule), b.Labels[0])

	rt.Metadata().Module = moduleName
	rt.Metadata().File = file
	rt.Metadata().Line = b.TypeRange.Start.Line
	rt.Metadata().Column = b.TypeRange.Start.Column

	err := decodeBody(ctx, file, b, rt)
	if err != nil {
		return fmt.Errorf("error creating resource '%s' in file %s", b.Type, err)
	}

	setDisabled(ctx, rt, b.Body, false)

	err = setDependsOn(ctx, rt, b.Body, dependsOn)
	if err != nil {
		de := ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = err.Error()

		return de
	}

	// we need to fetch the source so that we can process the child resources
	// "source" is the attribute but we need to read this manually
	src, diags := b.Body.Attributes["source"].Expr.Value(ctx)
	if diags.HasErrors() {
		return fmt.Errorf("unable to read source from module: %s", diags.Error())
	}

	// src could be a github module or a relative folder
	// first check if it is a folder, we need to make it absolute relative to the current file
	dir := path.Dir(file)
	moduleSrc := path.Join(dir, src.AsString())

	fi, err := os.Stat(moduleSrc)
	if err != nil || !fi.IsDir() {

		// is not a directory fetch from source using go getter
		gg := NewGoGetter()

		mp, err := gg.Get(src.AsString(), p.options.ModuleCache, false)
		if err != nil {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`unable to fetch remote module "%s" %s`, src.AsString(), err)

			return de
		}

		moduleSrc = mp
	}

	// create a new config and add the resources later
	moduleConfig := NewConfig()

	// modules should have their own context so that variables are not globally scoped
	subContext := buildContext(moduleSrc, p.registeredFunctions)

	err = p.parseDirectory(subContext, moduleSrc, moduleConfig)
	if err != nil {
		return err
	}

	rt.(*types.Module).SubContext = subContext

	// add the module
	c.addResource(rt, ctx, b.Body)

	// we need to add the module name to all the returned resources
	for _, r := range moduleConfig.Resources {
		// ensure the module name has the parent appended to it
		r.Metadata().Module = fmt.Sprintf("%s.%s", name, r.Metadata().Module)
		r.Metadata().Module = strings.TrimSuffix(r.Metadata().Module, ".")

		ctx, err := moduleConfig.getContext(r)
		if err != nil {
			panic("no body found for resource")
		}

		bdy, err := moduleConfig.getBody(r)
		if err != nil {
			panic("no body found for resource")
		}

		// set disabled
		setDisabled(ctx, r, bdy, rt.Metadata().Disabled)

		// depends on is a property of the embedded type we need to set this manually
		setDependsOn(ctx, rt, b.Body, dependsOn)

		c.addResource(r, ctx, bdy)
	}

	return nil
}

func (p *Parser) parseResource(ctx *hcl.EvalContext, c *Config, file string, b *hclsyntax.Block, moduleName string, dependsOn []string, disabled bool) error {

	var rt types.Resource
	var err error

	switch b.Type {
	case types.TypeResource:
		// if the type is resource there should be two labels, one for the type and one for the name
		if len(b.Labels) != 2 {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `"invalid formatting for 'resource' stanza, resources should have a name and a type, i.e. 'resource "type" "name" {}'`

			return de
		}

		name := b.Labels[1]
		if name == "resource" || name == "module" || name == "output" {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`invalid resource name "%s", resource, name, and output are reserved resource names`, name)

			return de
		}

		rt, err = p.registeredTypes.CreateResource(b.Labels[0], name)
		if err != nil {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf("unable to create resource '%s' %s", b.Type, err)

			return err
		}
	case types.TypeOutput:
		// if the type is output check there is one label
		if len(b.Labels) != 1 {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `invalid formatting for 'output' stanza, resources should have a name and a type, i.e. 'output "name" {}'`

			return de
		}

		name := b.Labels[0]
		if name == "resource" || name == "module" || name == "output" {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`invalid resource name "%s", resource, name, and output are reserved resource names`, name)

			return de
		}

		rt, err = p.registeredTypes.CreateResource(types.TypeOutput, name)
		if err != nil {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`unable to create output, this error should never happen %s`, err)

			return de
		}
	}

	rt.Metadata().Module = moduleName
	rt.Metadata().File = file
	rt.Metadata().Line = b.TypeRange.Start.Line
	rt.Metadata().Column = b.TypeRange.Start.Column

	err = decodeBody(ctx, file, b, rt)
	if err != nil {
		return fmt.Errorf("error creating resource '%s' in file %s: %s", b.Labels[0], file, err)
	}

	// disabled is a property of the embedded type we need to add this manually
	setDisabled(ctx, rt, b.Body, disabled)

	// depends on is a property of the embedded type we need to set this manually
	err = setDependsOn(ctx, rt, b.Body, dependsOn)
	if err != nil {
		de := ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to set depends_on, %s`, err)

		return de
	}

	// call the resources Parse function if set
	// if the config implements the processable interface call the resource process method
	if p, ok := rt.(types.Parsable); ok {
		err := p.Parse()
		if err != nil {
			de := ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`error parsing resource "%s" %s`, types.FQDNFromResource(rt).String(), err)

			return de
		}
	}

	err = c.addResource(rt, ctx, b.Body)
	if err != nil {
		de := ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to add resource "%s" to config %s`, types.FQDNFromResource(rt).String(), err)

		return de
	}

	return nil
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
	valMap := map[string]cty.Value{}

	// get the existing map
	if m, ok := ctx.Variables["variable"]; ok {
		valMap = m.AsValueMap()
	}

	valMap[key] = value

	ctx.Variables["variable"] = cty.ObjectVal(valMap)
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

	name := path[0]
	index := -1

	// is the path an array
	rg, _ := regexp.Compile(`(.*)\[([0-9])+\]`)
	if sm := rg.FindStringSubmatch(path[0]); len(sm) == 3 {
		name = sm[1]

		var convErr error
		index, convErr = strconv.Atoi(sm[2])
		if convErr != nil {
			panic(fmt.Sprintf("Index %s is not a number", sm[2]))
		}
	}

	// if not we need to create a map node if it does not exist
	// and recurse
	val, ok := root[name]
	if !ok {
		// do we have an array if so create a list
		if index >= 0 {
			// create a list with the correct length
			vals := make([]cty.Value, index+1)

			// need to set default values or the parser will panic
			for i, _ := range vals {
				vals[i] = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
			}

			val = cty.ListVal(vals)
		} else {
			// create a map node
			val = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
		}
	}

	if index >= 0 {

		updated := setListVariableFromPath(val.AsValueSlice(), path[1:], index, value)
		root[name] = cty.ListVal(updated)
	} else {
		updated := setMapVariableFromPath(val.AsValueMap(), path[1:], value)
		root[name] = cty.ObjectVal(updated)
	}

	return root
}

func setListVariableFromPath(root []cty.Value, path []string, index int, value cty.Value) []cty.Value {
	// we have a node but do we need to expand it in size?
	if index >= len(root) {
		root = append(root, make([]cty.Value, index+1-len(root))...)
	}

	val := root[index]
	if val == cty.NilVal {
		val = cty.ObjectVal(map[string]cty.Value{".keep": cty.BoolVal(true)})
	}

	updated := setMapVariableFromPath(val.AsValueMap(), path, value)
	root[index] = cty.ObjectVal(updated)

	// build a unique list of keys and types
	ul := map[string]cty.Type{}
	for _, m := range root {
		for k, v := range m.AsValueMap() {
			ul[k] = v.Type()
		}
	}

	// we need to normalize the map collection as cty does not allow inconsistent map keys
	for k, v := range ul {
		for i, m := range root {
			if _, ok := m.AsValueMap()[k]; !ok {
				val := root[i].AsValueMap()
				val[k] = cty.NullVal(v)
				root[i] = cty.ObjectVal(val)
			}
		}
	}

	return root
}

func buildContext(filePath string, customFunctions map[string]function.Function) *hcl.EvalContext {
	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{},
		Variables: map[string]cty.Value{},
	}

	valMap := map[string]cty.Value{}
	ctx.Variables["resource"] = cty.ObjectVal(valMap)

	ctx.Functions = getDefaultFunctions(filePath)

	// add the custom functions
	for k, v := range customFunctions {
		ctx.Functions[k] = v
	}

	return ctx
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
	res.Metadata().ResourceLinks = uniqueResources

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
// something = resource.mine.array.0.attr
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
			switch t.(type) {
			case hcl.TraverseAttr:
				strExpression += "." + t.(hcl.TraverseAttr).Name
			case hcl.TraverseIndex:
				strExpression += "[" + t.(hcl.TraverseIndex).Key.AsBigFloat().String() + "]"
			}
		}
	}

	// add to the references collection and replace with a nil value
	// we will resolve these references before processing
	return strExpression, nil
}

// recurses throught destination object and returns the type of the field marked by path
// e.g path "volume[1].source" is string
func findTypeFromInterface(path string, s interface{}) string {
	// strip the indexes as we are doing the lookup on a empty struct
	re, _ := regexp.Compile(`\[[0-9]+\]`)
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

// UnmarshalJSON parses a JSON string from a serialized Config and returns a
// valid Config.
func (p *Parser) UnmarshalJSON(d []byte) (*Config, error) {
	conf := NewConfig()

	var objMap map[string]*json.RawMessage
	err := json.Unmarshal(d, &objMap)
	if err != nil {
		return nil, err
	}

	var rawMessagesForResources []*json.RawMessage
	err = json.Unmarshal(*objMap["resources"], &rawMessagesForResources)
	if err != nil {
		return nil, err
	}

	for _, m := range rawMessagesForResources {
		mm := map[string]interface{}{}
		err := json.Unmarshal(*m, &mm)
		if err != nil {
			return nil, err
		}

		r, err := p.registeredTypes.CreateResource(mm["type"].(string), mm["name"].(string))
		if err != nil {
			return nil, err
		}

		resData, _ := json.Marshal(mm)

		json.Unmarshal(resData, r)
		conf.addResource(r, nil, nil)
	}

	return conf, nil
}
