package hclconfig

import (
	"errors"
	"fmt"
	"os"
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
	Variables      map[string]string
	VariablesFiles []string
	ModuleCache    string
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
		ModuleCache: cacheDir,
	}
}

// Parser can parse HCL configuration files
type Parser struct {
	options         ParserOptions
	registeredTypes types.RegisteredTypes
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

	return c, nil
}

// parseFile loads variables and resources from the given file
func (p *Parser) parseFile(
	ctx *hcl.EvalContext,
	file string,
	c *Config,
	variables map[string]string,
	variablesFile []string) error {

	setVariables(ctx, variables)
	for _, vf := range variablesFile {
		err := loadVariablesFromFile(ctx, vf)
		if err != nil {
			return err
		}
	}

	// This must be done before any other process as the resources
	// might reference the variables
	err := p.parseVariablesInFile(ctx, file, c)
	if err != nil {
		return err
	}

	err = p.parseResourcesInFile(ctx, file, c, "", false, []string{})
	if err != nil {
		return err
	}

	err = p.parseOutputsInFile(ctx, file, false, c)
	if err != nil {
		return err
	}

	return nil
}

// loadVariablesFromFile loads variable values from a file
func loadVariablesFromFile(ctx *hcl.EvalContext, path string) error {
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
func setVariables(ctx *hcl.EvalContext, vars map[string]string) {
	// first any vars defined as environment variables
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "SY_VAR_") {
			parts := strings.Split(e, "=")

			if len(parts) == 2 {
				key := strings.Replace(parts[0], "SY_VAR_", "", -1)
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

// parseOutputsInFile parses a hcl file and adds any found resources to the config
func (p *Parser) parseOutputsInFile(ctx *hcl.EvalContext, file string, disabled bool, c *Config) error {
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
		case string(types.TypeOutput):
			v := (&types.Output{}).New(b.Labels[0])

			err := decodeBody(ctx, file, b, v)
			if err != nil {
				return err
			}

			setDisabled(v, disabled)

			// if disabled do not add to the resources list
			if !v.Info().Disabled {
				c.AddResource(v)
			}
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
		if types.ResourceType(b.Type) != types.TypeVariable && types.ResourceType(b.Type) != types.TypeOutput {
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
		}
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

func buildContext(filePath string) *hcl.EvalContext {

	var EnvFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "env",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(os.Getenv(args[0].AsString())), nil
		},
	})

	//var HomeFunc = function.New(&function.Spec{
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		return cty.StringVal(utils.HomeFolder()), nil
	//	},
	//})

	//var ShipyardFunc = function.New(&function.Spec{
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		return cty.StringVal(utils.ShipyardHome()), nil
	//	},
	//})

	//var DockerIPFunc = function.New(&function.Spec{
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		return cty.StringVal(utils.GetDockerIP()), nil
	//	},
	//})

	//var DockerHostFunc = function.New(&function.Spec{
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		return cty.StringVal(utils.GetDockerHost()), nil
	//	},
	//})

	//var ShipyardIPFunc = function.New(&function.Spec{
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		ip, _ := utils.GetLocalIPAndHostname()
	//		return cty.StringVal(ip), nil
	//	},
	//})

	//var KubeConfigFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "k8s_config",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		_, kcp, _ := utils.CreateKubeConfigPath(args[0].AsString())
	//		return cty.StringVal(kcp), nil
	//	},
	//})

	//var KubeConfigDockerFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "k8s_config_docker",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		_, _, kcp := utils.CreateKubeConfigPath(args[0].AsString())
	//		return cty.StringVal(kcp), nil
	//	},
	//})

	//var FileFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "path",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		// convert the file path to an absolute
	//		fp := ensureAbsolute(args[0].AsString(), filePath)

	//		// read the contents of the file
	//		d, err := ioutil.ReadFile(fp)
	//		if err != nil {
	//			return cty.StringVal(""), err
	//		}

	//		return cty.StringVal(string(d)), nil
	//	},
	//})

	//var DataFuncWithPerms = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "path",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//		{
	//			Name:             "permissions",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//			AllowNull:        true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		perms := os.ModePerm
	//		output, err := strconv.ParseInt(args[1].AsString(), 8, 64)
	//		if err != nil {
	//			return cty.StringVal(""), fmt.Errorf("Invalid file permission")
	//		}

	//		perms = os.FileMode(output)
	//		return cty.StringVal(utils.GetDataFolder(args[0].AsString(), perms)), nil
	//	},
	//})

	//var DataFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "path",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		perms := os.FileMode(os.ModePerm)
	//		return cty.StringVal(utils.GetDataFolder(args[0].AsString(), perms)), nil
	//	},
	//})

	//var ClusterAPIFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "name",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		conf, _ := utils.GetClusterConfig(args[0].AsString())

	//		return cty.StringVal(conf.APIAddress(utils.LocalContext)), nil
	//	},
	//})

	//var ClusterPortFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "name",
	//			Type:             cty.String,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.Number),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		conf, _ := utils.GetClusterConfig(args[0].AsString())

	//		return cty.NumberIntVal(int64(conf.RemoteAPIPort)), nil
	//	},
	//})

	//var LenFunc = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "var",
	//			Type:             cty.DynamicPseudoType,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.Number),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		if len(args) == 1 && args[0].Type().IsCollectionType() || args[0].Type().IsTupleType() {
	//			i := args[0].ElementIterator()
	//			if i.Next() {
	//				return args[0].Length(), nil
	//			}
	//		}

	//		return cty.NumberIntVal(0), nil
	//	},
	//})

	//var DirFunc = function.New(&function.Spec{
	//	Type: function.StaticReturnType(cty.String),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		s, err := filepath.Abs(filePath)

	//		return cty.StringVal(filepath.Dir(s)), err
	//	},
	//})

	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{},
		Variables: map[string]cty.Value{},
	}

	valMap := map[string]cty.Value{}
	ctx.Variables["resources"] = cty.ObjectVal(valMap)

	//ctx.Functions["len"] = LenFunc
	ctx.Functions["env"] = EnvFunc
	//ctx.Functions["k8s_config"] = KubeConfigFunc
	//ctx.Functions["k8s_config_docker"] = KubeConfigDockerFunc
	//ctx.Functions["home"] = HomeFunc
	//ctx.Functions["shipyard"] = ShipyardFunc
	//ctx.Functions["file"] = FileFunc
	//ctx.Functions["data"] = DataFunc
	//ctx.Functions["data_with_permissions"] = DataFuncWithPerms
	//ctx.Functions["docker_ip"] = DockerIPFunc
	//ctx.Functions["docker_host"] = DockerHostFunc
	//ctx.Functions["shipyard_ip"] = ShipyardIPFunc
	//ctx.Functions["cluster_api"] = ClusterAPIFunc
	//ctx.Functions["cluster_port"] = ClusterPortFunc
	//ctx.Functions["file_dir"] = DirFunc

	return ctx
}

func copyContext(path string, ctx *hcl.EvalContext) *hcl.EvalContext {
	newCtx := buildContext(path)
	newCtx.Variables = ctx.Variables

	return newCtx
}

func decodeBody(ctx *hcl.EvalContext, path string, b *hclsyntax.Block, p interface{}) error {
	dr := getDependentResources(b, ctx, p, "")

	diag := gohcl.DecodeBody(b.Body, ctx, p)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	// set the dependent resources
	p.(types.Resource).Info().ResouceLinks = dr

	return nil
}

// Recurively checks the fields and blocks on the resource to identify links to other resources
// i.e. resource.container.network[0].name
// when a link is found it is replaced with an empty value of the correct type and the
// dependent resources are returned to be processed later
func getDependentResources(b *hclsyntax.Block, ctx *hcl.EvalContext, resource interface{}, attribute string) map[string]string {
	references := map[string]string{}

	breadcrumb := attribute

	for _, a := range b.Body.Attributes {
		// look for scope traversal expressions
		switch a.Expr.(type) {
		case *hclsyntax.ScopeTraversalExpr:
			ste := a.Expr.(*hclsyntax.ScopeTraversalExpr)
			strExpression := ""
			for i, t := range ste.Traversal {
				if i == 0 {
					strExpression += t.(hcl.TraverseRoot).Name
				} else {
					// does this exist in the context
					strExpression += "." + t.(hcl.TraverseAttr).Name
				}
			}

			// add to the references collection and replace with a nil value
			// only when starts with resources
			if strings.HasPrefix(strExpression, "resources.") {
				// we will resolve these references before processing
				attrRef := breadcrumb + "." + a.Name
				references[attrRef] = strExpression

				// we need to find the type of the field we will eventually deserialze into so that
				// we can replace this with a null value for now
				t := findTypeFromInterface(attrRef, resource)

				switch t {
				case "string":
					a.Expr = &hclsyntax.LiteralValueExpr{Val: cty.StringVal(""), SrcRange: a.SrcRange}
				case "int":
					a.Expr = &hclsyntax.LiteralValueExpr{Val: cty.NumberIntVal(0), SrcRange: a.SrcRange}

				default:
					fmt.Println(t)
				}
			}
		}
	}

	// we need to keep a count of the block
	blockIndex := map[string]int{}
	for _, b := range b.Body.Blocks {
		if _, ok := blockIndex[b.Type]; ok {
			blockIndex[b.Type]++
		} else {
			blockIndex[b.Type] = 0
		}

		ref := fmt.Sprintf("%s.%s[%d]", breadcrumb, b.Type, blockIndex[b.Type])
		ref = strings.TrimPrefix(ref, ".")
		cr := getDependentResources(b, ctx, resource, ref)
		for k, v := range cr {
			references[k] = v
		}
	}

	return references
}

// recurses throught destination object and returns the type of the field marked by path
// e.g path "volume[1].source" is string
func findTypeFromInterface(path string, s interface{}) string {
	// strip the indexes as we are doing the lookup on a empty struct
	re, _ := regexp.Compile("\\[[0-9]+\\]")
	stripped := re.ReplaceAllString(path, "")

	value := reflect.ValueOf(s).Type()
	val, found := lookup.LookupType(value, strings.Split(stripped, "."), false, []string{"hcl", "json"})

	if found {
		return val.Name()
	}

	return ""
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

// ParseFolder for Resource, Blueprint, and Variable files
// The onlyResources parameter allows you to specify that the parser
// moduleName is the name of the module, this should be set to a blank string for the root module
// disabled sets the disabled flag on all resources, this is used when parsing a module that
//
//	has the disabled flag set
//
// only reads resource files and will ignore Blueprint and Variable files.
// This is useful when recursively parsing such as when reading Modules
//func ParseFolder(
//	folder string,
//	c *resources.Config,
//	onlyResources bool,
//	moduleName string,
//	disabled bool,
//	dependsOn []string,
//	variables map[string]string,
//	variablesFile string) error {
//
//	rootContext = buildContext(folder)
//
//	err := parseFolder(
//		rootContext,
//		folder,
//		c,
//		onlyResources,
//		moduleName,
//		disabled,
//		dependsOn,
//		variables,
//		variablesFile,
//	)
//
//	if err != nil {
//		return err
//	}
//
//	return parseReferences(c)
//
//}

//func parseFolder(
//	ctx *hcl.EvalContext,
//	folderPath string,
//	config *resources.Config,
//	onlyResources bool,
//	moduleName string,
//	disabled bool,
//	dependsOn []string,
//	variables map[string]string,
//	variablesFile string) error {
//
//	absolutePath, _ := filepath.Abs(folderPath)
//
//	if ctx == nil {
//		panic("Context nil")
//	}
//
//	if ctx.Functions == nil {
//		panic("Context Functions nil")
//	}
//
//	// load the variables from the root of the blueprint
//	if !onlyResources {
//		variableFiles, err := filepath.Glob(path.Join(absolutePath, "*.vars"))
//		if err != nil {
//			return err
//		}
//
//		for _, f := range variableFiles {
//			err := loadVariablesFromFile(ctx, f)
//			if err != nil {
//				return err
//			}
//		}
//
//		// load variables from any custom files set on the command line
//		if variablesFile != "" {
//			err := loadVariablesFromFile(ctx, variablesFile)
//			if err != nil {
//				return err
//			}
//		}
//
//		// setup any variables which are passed as environment variables or in the collection
//		setVariables(ctx, variables)
//
//		yardFilesMD, err := filepath.Glob(path.Join(absolutePath, "README.md"))
//		if err != nil {
//			return err
//		}
//
//		if len(yardFilesMD) > 0 {
//			err := parseYardMarkdown(yardFilesMD[0], config)
//			if err != nil {
//				return err
//			}
//		}
//	}
//
//	// We need to do a two pass parsing, first we check if there are any
//	// default variables which should be added to the collection
//	err := parseVariables(ctx, absolutePath, config)
//	if err != nil {
//		return err
//	}
//
//	// Parse Resource files from the current folder
//	err = parseResources(ctx, absolutePath, config, moduleName, disabled, dependsOn)
//	if err != nil {
//		return err
//	}
//
//	// Finally parse the outputs
//	err = parseOutputs(ctx, absolutePath, disabled, config)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

//	func getModuleFiles(source, dest string) error {
//		// check to see if a folder exists at the destination and exit if exists
//		_, err := os.Stat(dest)
//		if err == nil {
//			return nil
//		}
//
//		pwd, err := os.Getwd()
//		if err != nil {
//			return err
//		}
//
//		c := &getter.Client{
//			Ctx:     context.Background(),
//			Src:     source,
//			Dst:     dest,
//			Pwd:     pwd,
//			Mode:    getter.ClientModeAny,
//			Options: []getter.ClientOption{},
//		}
//
//		err = c.Get()
//		if err != nil {
//			return xerrors.Errorf("unable to fetch files from %s: %w", source, err)
//		}
//
//		return nil
//	}
