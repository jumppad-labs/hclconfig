package parser

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/functions"
	"github.com/jumppad-labs/xcl/internal/modules"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/silas/dag"
	"github.com/zclconf/go-cty/cty/function"
)

type ResourceTypeNotExistError struct {
	Type string
	File string
}

func (r ResourceTypeNotExistError) Error() string {
	return fmt.Sprintf("Resource type %s defined in file %s, is not a registered resource.", r.Type, r.File)
}

// parsed holds resources and their HCL bodies during the parsing phase.
// This is an internal working structure used only within the Parser.
// After parsing completes, resources are transferred to State (without bodies).
type parsed struct {
	resources map[string]any             // FQRN -> resource instance
	bodies    map[string]*hclsyntax.Body // FQRN -> HCL body for later decoding

	// moduleSources records the module source directories already entered,
	// resolved to an absolute path with symlinks evaluated. A module whose
	// source directly or indirectly includes itself would otherwise recurse
	// until the stack is exhausted, which is a crash rather than a reportable
	// problem.
	moduleSources map[string]bool
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
	// default registry to use when fetching modules
	DefaultRegistry string
	// credentials to use with the registries
	RegistryCredentials map[string]string

	// Logger function for plugin discovery logging (optional)
	Logger logger.Logger

	// ModuleRegistry is the registry of modules to use for this parser
	// when downloading modules.
	ModuleRegistry *modules.ModuleRegistry

	// PluginRegistry is the registry of plugins to use for this parser.
	// This should be provided by Config when using the Config.Apply/Validate API.
	// For standalone Parser usage, create and configure the PluginRegistry yourself.
	PluginRegistry *registry.PluginRegistry

	// ProviderResolver overrides how provider adapters are looked up during the resource
	// lifecycle walk (Create/Refresh/Changed/Update/Destroy). Defaults to PluginRegistry.
	// Primarily useful for testing lifecycle/ordering behavior without a real plugin registry.
	ProviderResolver ProviderResolver

	// StateStore is the state store to use for loading previous state.
	// and saving new state.
	StateStore state.StateStore

	// OnParserEvent is an optional callback function that is called when parser events occur.
	// This can be used for metrics collection, logging, debugging, or other monitoring purposes.
	OnParserEvent func(ParserEvent)

	CustomFunctions map[string]function.Function
}

// DefaultOptions returns a ParserOptions object with the
// ModuleCache set to the default directory of $HOME/.xcl/cache
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

	os.MkdirAll(cacheDir, os.ModePerm)

	// Default plugin directories
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "."
	}

	cacheDir = filepath.Join(homeDir, ".xcl", "cache")

	logger := logger.NewStdOutLogger()

	return &ParserOptions{
		ModuleCache:       cacheDir,
		VariableEnvPrefix: "HCL_VAR_",
		Logger:            logger,
	}
}

// Parser can parse HCL configuration files
type Parser struct {
	options          ParserOptions
	customFunctions  map[string]function.Function
	stateStore       state.StateStore
	pluginRegistry   *registry.PluginRegistry
	providerResolver ProviderResolver
	parsedResources  *parsed // Working storage during parsing
}

// NewParser creates a new parser with the given options
// if options are nil, default options are used
func NewParser(options *ParserOptions) *Parser {
	o := options
	if o == nil {
		o = DefaultOptions()
	}

	// Create logger if there is not one set
	if o.Logger == nil {
		o.Logger = &logger.StdOutLogger{}
	}

	p := &Parser{
		options:         *o,
		customFunctions: map[string]function.Function{},
	}

	// Parser should never create plugin registry or state store - these are owned by Config
	// and passed in via options. If not provided, they will be nil and operations will skip
	// plugin/state functionality.
	p.pluginRegistry = o.PluginRegistry
	p.stateStore = o.StateStore

	p.providerResolver = o.ProviderResolver
	if p.providerResolver == nil {
		p.providerResolver = p.pluginRegistry
	}

	if o.CustomFunctions != nil {
		p.customFunctions = o.CustomFunctions
	}

	return p
}

// Apply parses HCL configuration from multiple paths, calls the provider
// lifecycle for every resource and returns State with decoded resources.
// This is the main entry point for applying configuration.
//
// Parameters:
//   - paths: one or more file or directory paths to parse
//
// The parsing process:
//  1. Load previous state from StateStore (if exists)
//  2. Parse all HCL files from the given paths
//  3. Build a DAG based on resource dependencies
//  4. Walk the DAG in dependency order
//  5. Decode each resource body (HCL → Go structs)
//  6. Compare with previousState and call Create/Update on plugins
//
// Returns the new State containing all parsed resources.
func (p *Parser) Apply(paths ...string) (*state.State, error) {
	currentState, previousState, err := p.parseAndValidate(paths...)
	if err != nil {
		return nil, err
	}

	ce := errors.NewConfigError()

	// Get functions for HCL context
	functions := p.getFunctions()

	// Always walk the DAG to decode resources (fills in their fields from HCL)
	// This decodes interpolations and resolves dependencies regardless of plugin execution
	errs := p.walk(currentState, previousState, functions)
	if len(errs) > 0 {
		for _, e := range errs {
			ce.AppendError(e)
		}
		return nil, ce
	}

	return currentState, nil
}

// Validate parses and validates the configuration discovered from paths without
// resolving it: no body is decoded, no dependency graph is walked and no provider
// is reached. A nil error means the configuration is valid.
func (p *Parser) Validate(paths ...string) error {
	_, _, err := p.parseAndValidate(paths...)
	return err
}

// parseAndValidate reads every file discovered from paths, then validates the
// result as a whole before any of it is acted upon. It performs no resolution:
// no body is decoded, no dependency graph is walked and no provider is reached.
//
// It returns the state holding the parsed resources along with the previously
// stored state, or every problem found. A configuration that does not parse is
// never validated, and a configuration that does not validate is never returned,
// so a caller that receives no error holds a configuration worth acting on.
func (p *Parser) parseAndValidate(paths ...string) (*state.State, *state.State, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("at least one path is required")
	}

	// Load previous state from store (for comparison during Create/Update)
	var previousState *state.State
	if p.stateStore != nil && p.stateStore.Exists() {
		var err error
		previousState, err = p.stateStore.Load()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load previous state: %w", err)
		}
	}
	if previousState == nil {
		previousState = state.NewState()
	}

	// Create new state for current parse
	currentState := state.NewState()

	// Initialize parsed resources container
	p.parsedResources = &parsed{
		resources:     map[string]any{},
		bodies:        map[string]*hclsyntax.Body{},
		moduleSources: map[string]bool{},
	}

	ce := errors.NewConfigError()

	// Auto-discover .vars files from the root directories of all paths
	// These have lower precedence than manually specified VariablesFiles
	discoveredVarsFiles, err := findVarsFiles(paths...)
	if err != nil {
		ce.AppendError(errors.NewParserError("", 0, 0,
			fmt.Sprintf("error finding .vars files: %s", err)))
		return nil, nil, ce
	}

	// Merge discovered vars files with manually specified ones
	// Discovered files come first (lower precedence), then manually specified (higher precedence)
	mergedVarsFiles := []string{}

	// Add discovered files only if not already in manually specified list
	for _, discovered := range discoveredVarsFiles {
		found := false
		for _, manual := range p.options.VariablesFiles {
			if discovered == manual {
				found = true
				break
			}
		}
		if !found {
			mergedVarsFiles = append(mergedVarsFiles, discovered)
		}
	}

	// Add all manually specified files (these have higher precedence)
	mergedVarsFiles = append(mergedVarsFiles, p.options.VariablesFiles...)

	// Update options with merged list
	p.options.VariablesFiles = mergedVarsFiles

	// Get all the xcl files from the paths
	files, err := findXclFiles(paths...)
	if err != nil {
		ce.AppendError(errors.NewParserError("", 0, 0,
			fmt.Sprintf("error finding .xcl files: %s", err)))
		return nil, nil, ce
	}

	// Parse all files
	for _, file := range files {
		errs := p.parseResourcesInFile(file, "")
		for _, e := range errs {
			ce.AppendError(e)
		}
	}

	if len(ce.Errors) > 0 {
		return nil, nil, ce
	}

	// Validate the configuration as a whole before any of it is acted upon.
	// Every resource from every file is parsed by this point and no body has
	// been decoded, which is what lets validation judge the configuration
	// without resolving it.
	for _, e := range p.validate() {
		ce.AppendError(e)
	}

	if len(ce.Errors) > 0 {
		return nil, nil, ce
	}

	// Move parsed resources into currentState
	for _, resource := range p.parsedResources.resources {
		if err := currentState.AppendResource(resource); err != nil {
			return nil, nil, fmt.Errorf("failed to add resource to state: %w", err)
		}
	}

	return currentState, previousState, nil
}

// parseResourcesInFile parses a hcl file and adds any found resources to the config
func (p *Parser) parseResourcesInFile(file string, module string) []error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseXCLFile(file)
	if diag.HasErrors() {
		// a single malformed input can produce several diagnostics, report every
		// one of them rather than only the first
		errs := []error{}
		for _, d := range diag {
			errs = append(errs, errors.NewParserErrorFromHCLDiag(d, file))
		}

		return errs
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		// this should never happen, body should always be a hclsyntax.Body
		panic("Error getting body")
	}

	// gather every problem in the file rather than returning at the first, so an
	// author sees all of them at once
	blockErrors := []error{}

	for _, b := range body.Blocks {

		// check the resource has a name
		if len(b.Labels) == 0 {
			de := errors.NewParserError(
				file,
				b.TypeRange.Start.Line,
				b.TypeRange.Start.Column,
				fmt.Sprintf("resource '%s' has no name, please specify resources using the syntax 'resource_type \"name\" {}'", b.Type),
			)

			blockErrors = append(blockErrors, de)
			continue
		}

		// create the registered type if not a variable or output
		// variables and outputs are processed in a separate run
		switch b.Type {
		case resources.TypeModule:
			errs := p.parseModule(file, b, module)
			blockErrors = append(blockErrors, errs...)
		case resources.TypeVariable:
			fallthrough
		case resources.TypeOutput:
			fallthrough
		case types.TypeResource:
			err := p.parseResource(file, b, module)
			if err != nil {
				blockErrors = append(blockErrors, err)
			}
		default:
			de := errors.NewParserError(
				file,
				b.TypeRange.Start.Line,
				b.TypeRange.Start.Column,
				fmt.Sprintf("unable to process stanza '%s' in file %s at %d,%d , only 'variable', 'resource', 'module', and 'output' are valid stanza blocks", b.Type, file, b.Range().Start.Line, b.Range().Start.Column),
			)

			blockErrors = append(blockErrors, de)
		}
	}

	if len(blockErrors) > 0 {
		return blockErrors
	}

	return nil
}

func (p *Parser) parseResource(file string, b *hclsyntax.Block, moduleName string) error {
	var rt any
	var err error

	switch b.Type {
	case types.TypeResource:
		// If the type is resource there should be two labels, one for the type and one for the name
		if len(b.Labels) != 2 {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `"invalid format for 'resource', resources should have a name and a type, i.e. 'resource "type" "name" {}'`

			return de
		}

		// Check the resource name is valid
		name := b.Labels[1]
		if err := validateResourceName(name); err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = de.Error()

			return de
		}

		// Create resource type using plugin registry
		rt, err = p.pluginRegistry.CreateResource(b.Labels[0], name)
		if err != nil {
			de := errors.NewParserError(
				file,
				b.TypeRange.Start.Line,
				b.TypeRange.Start.Column,
				fmt.Sprintf("unable to create resource '%s' of type '%s': %s", name, b.Labels[0], err),
			)
			return de
		}

	case resources.TypeOutput:
		// If the type is output check there is one label
		if len(b.Labels) != 1 {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `invalid formatting for 'output' stanza, resources should have a name and a type, i.e. 'output "name" {}'`

			return de
		}

		name := b.Labels[0]
		if err := validateResourceName(name); err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = err.Error()

			return de
		}

		rt, err = p.createBuiltinResource(resources.TypeOutput, name)
		if err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`unable to create output, this error should never happen %s`, err)

			return de
		}
	case resources.TypeVariable:
		// If the type is variable check there is one label
		if len(b.Labels) != 1 {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `invalid formatting for 'variable' stanza, resources should have a name and a type, i.e. 'variable "name" {}'`

			return de
		}

		name := b.Labels[0]
		if err := validateResourceName(name); err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = err.Error()

			return de
		}

		rt, err = p.createBuiltinResource(resources.TypeVariable, name)
		if err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`unable to create variable, this error should never happen %s`, err)

			return de
		}
	}

	// We now have an entity, get the meta
	rtMeta, err := types.GetMeta(rt)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("unable to get resource meta for resource %s: %s", b.Labels[0], err)
		return de
	}

	rtMeta.Module = moduleName
	rtMeta.File = file
	rtMeta.Line = b.TypeRange.Start.Line
	rtMeta.Column = b.TypeRange.Start.Column

	// Set the ID from the FQRN
	fqrn := resources.FQRNFromResource(rt)
	rtMeta.ID = fqrn.String()

	// We now need to get all the dependent resources for this resource
	// so that we can build the dependency graph
	err = p.getUniqueResourceLinks(rt, b)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("error creating resource '%s' in file %s: %s", b.Labels[0], file, err)
		return de
	}

	// if we have an output, get the description
	// this is needed during parsing as the value may not be set during walk
	if err == nil && rtMeta.Type == resources.TypeOutput && b.Body.Attributes["description"] != nil {
		desc, diags := b.Body.Attributes["description"].Expr.Value(nil)
		if !diags.HasErrors() {
			rt.(*resources.Output).Description = desc.AsString()
		}
	}

	// TODO: It may be posible to remove this code as depends_on is only
	// designed to provide graph dependencies, and we have already processed those above
	// however, if depends on is not serialized to the state, this could cause issues.
	// code removed for now until tests are passing.

	// depends on is a property of the embedded type we need to set this manually
	//err = setDependsOn(nil, rt, b.Body, dependsOn)
	//if err != nil {
	//	de := &errors.ParserError{}
	//	de.Line = b.TypeRange.Start.Line
	//	de.Column = b.TypeRange.Start.Column
	//	de.Filename = file
	//	de.Message = fmt.Sprintf(`unable to set depends_on, %s`, err)

	//	return de
	//}

	// add the resource to the cache
	p.parsedResources.bodies[rtMeta.ID] = b.Body
	p.parsedResources.resources[rtMeta.ID] = rt

	return nil
}

// parseModule creates a shell for a module block, mirroring the non-eager
// shell-creation path parseResource uses for other block types. It does not
// decode the module's body at parse time; Variables and Disabled remain
// zero-valued on the shell until the Phase-2 DAG walk decodes them. The
// module's source directory is resolved and recursed into here (Phase 1.2)
// so that the module's child resources are discovered and scoped under the
// module's own instance name.
func (p *Parser) parseModule(file string, b *hclsyntax.Block, parentModule string) []error {
	// If the type is module there should be one label for the instance name
	if len(b.Labels) != 1 {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = `invalid formatting for 'module' stanza, resources should have a name and a type, i.e. 'module "name" {}'`

		return []error{de}
	}

	name := b.Labels[0]
	if err := validateResourceName(name); err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = err.Error()

		return []error{de}
	}

	rt, err := p.createBuiltinResource(resources.TypeModule, name)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to create module, this error should never happen %s`, err)

		return []error{de}
	}

	// We now have an entity, get the meta
	rtMeta, err := types.GetMeta(rt)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("unable to get resource meta for resource %s: %s", b.Labels[0], err)
		return []error{de}
	}

	rtMeta.Module = parentModule
	rtMeta.File = file
	rtMeta.Line = b.TypeRange.Start.Line
	rtMeta.Column = b.TypeRange.Start.Column

	// Set the ID from the FQRN
	fqrn := resources.FQRNFromResource(rt)
	rtMeta.ID = fqrn.String()

	// We now need to get all the dependent resources for this module so that
	// we can build the dependency graph; this walks the module's source,
	// variables, and disabled attributes the same way it does for any other
	// resource type
	err = p.getUniqueResourceLinks(rt, b)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("error creating resource '%s' in file %s: %s", b.Labels[0], file, err)
		return []error{de}
	}

	// add the module to the cache
	p.parsedResources.bodies[rtMeta.ID] = b.Body
	p.parsedResources.resources[rtMeta.ID] = rt

	// Resolve the module's source as a local directory relative to the file
	// that declared it, and recurse into that directory's .xcl files, scoping
	// every resource discovered there under this module's own instance name.
	sourceAttr, ok := b.Body.Attributes["source"]
	if !ok {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`module '%s' has no 'source' attribute`, name)
		return []error{de}
	}

	sourceVal, diags := sourceAttr.Expr.Value(nil)
	if diags.HasErrors() {
		de := &errors.ParserError{}
		de.Line = sourceAttr.SrcRange.Start.Line
		de.Column = sourceAttr.SrcRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to resolve 'source' for module '%s': %s`, name, diags.Error())
		return []error{de}
	}

	sourceDir := filepath.Join(filepath.Dir(file), sourceVal.AsString())

	moduleInstanceName := name
	if parentModule != "" {
		moduleInstanceName = parentModule + "." + name
	}

	// Resolve the source to a canonical path so that a module reached by two
	// different spellings of the same directory is recognised as the same
	// source. A source that cannot be resolved is reported against the module,
	// like any other module whose contents cannot be obtained.
	canonicalSource, err := canonicalPath(sourceDir)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to obtain contents for module '%s' source '%s': %s`, name, sourceDir, err)
		return []error{de}
	}

	// A module whose source transitively includes itself would recurse until
	// the stack is exhausted. Report it against the module instead.
	if p.parsedResources.moduleSources[canonicalSource] {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`module '%s' source '%s' includes itself`, name, sourceDir)
		return []error{de}
	}

	p.parsedResources.moduleSources[canonicalSource] = true
	defer delete(p.parsedResources.moduleSources, canonicalSource)

	childFiles, err := findXclFiles(sourceDir)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to discover files for module '%s' source '%s': %s`, name, sourceDir, err)
		return []error{de}
	}

	moduleErrors := []error{}
	for _, childFile := range childFiles {
		moduleErrors = append(moduleErrors, p.parseResourcesInFile(childFile, moduleInstanceName)...)
	}

	if len(moduleErrors) > 0 {
		return moduleErrors
	}

	return nil
}

// getUniqueResourceLinks gets all the dependent resources for a resource
// these are either manaully set using the depends_on attribute or automatically
// inferred from the interpolations in the resource
func (p *Parser) getUniqueResourceLinks(resource any, b *hclsyntax.Block) error {
	dr, err := p.getDependentResources(resource, b)
	if err != nil {
		return err
	}

	for _, d := range dr {
		if err := types.AppendUniqueDependency(resource, d); err != nil {
			return fmt.Errorf("failed to add dependency %s: %w", d, err)
		}
	}

	body := b.Body
	if body == nil {
		// no body, nothing to do
		return nil
	}

	// now add the manually defined depends_on dependencies
	// depends_on does not support interpolation so use a nil context
	if attr, ok := body.Attributes["depends_on"]; ok {
		dependsOnVal, diags := attr.Expr.Value(&hcl.EvalContext{})
		if diags.HasErrors() {
			return fmt.Errorf("unable to read depends_on attribute: %s", diags.Error())
		}

		// depends on is a slice of string
		dependsOnSlice := dependsOnVal.AsValueSlice()
		for _, d := range dependsOnSlice {
			fqdn, err := resources.ParseFQRN(d.AsString())
			if err != nil {
				return fmt.Errorf("invalid dependency %s, %s", d.AsString(), err)
			}

			if err := types.AppendUniqueDependency(resource, fqdn.String()); err != nil {
				return fmt.Errorf("failed to add dependency %s: %w", d, err)
			}
		}
	}

	return nil
}

// getDependentResources recursively checks the fields and blocks on the resource to identify links to other resources
// i.e. resource.container.foo.network[0].name
// This enables automatic building of the dependency graph
func (p *Parser) getDependentResources(resource any, b *hclsyntax.Block) ([]string, error) {
	references := []string{}

	// Process all attributes in the block
	for _, a := range b.Body.Attributes {
		refs, err := processExpr(a.Expr)
		if err != nil {
			return nil, errors.NewParserError(
				b.Body.SrcRange.Filename,
				b.Body.SrcRange.Start.Line,
				b.Body.SrcRange.Start.Column,
				fmt.Sprintf("unable to process attribute %s: %s", a.Name, err),
			)
		}

		references = append(references, refs...)
	}

	// Process nested blocks recursively
	blockIndex := map[string]int{}
	for _, block := range b.Body.Blocks {
		if _, ok := blockIndex[block.Type]; ok {
			blockIndex[block.Type]++
		} else {
			blockIndex[block.Type] = 0
		}

		// Recursively get dependencies from nested blocks
		cr, err := p.getDependentResourcesFromBlock(block)
		if err != nil {
			return nil, err
		}

		references = append(references, cr...)
	}

	// Check for cyclical dependencies
	rMeta, err := types.GetMeta(resource)
	if err != nil {
		return references, nil // Skip cycle check if resource doesn't have metadata
	}

	for _, dep := range references {
		// Check if this dependency would create a cycle
		// Look up the dependency in parsedResources
		if p.parsedResources != nil {
			if depResource, ok := p.parsedResources.resources[dep]; ok {
				depMeta, err := types.GetMeta(depResource)
				if err != nil {
					continue // Skip if dependency doesn't have metadata
				}

				// Check if the dependency's links contain a reference back to us
				for _, cdep := range depMeta.Links {
					fqrn, err := resources.ParseFQRN(cdep)
					if err != nil {
						continue
					}
					fqrn.Attribute = ""

					// Check for direct cycle
					if rMeta.Name == fqrn.Resource &&
						rMeta.Type == fqrn.Type &&
						rMeta.Module == fqrn.Module {
						return nil, errors.NewParserError(
							b.Body.SrcRange.Filename,
							b.Body.SrcRange.Start.Line,
							b.Body.SrcRange.Start.Column,
							fmt.Sprintf("'%s' depends on '%s' which creates a cyclical dependency", rMeta.ID, depMeta.ID),
						)
					}
				}
			}
		}
	}

	return references, nil
}

// getDependentResourcesFromBlock extracts dependencies from a nested block
func (p *Parser) getDependentResourcesFromBlock(b *hclsyntax.Block) ([]string, error) {
	references := []string{}

	// Process attributes in the block
	for _, a := range b.Body.Attributes {
		refs, err := processExpr(a.Expr)
		if err != nil {
			return nil, fmt.Errorf("unable to process attribute %s: %w", a.Name, err)
		}
		references = append(references, refs...)
	}

	// Process nested blocks recursively
	for _, block := range b.Body.Blocks {
		cr, err := p.getDependentResourcesFromBlock(block)
		if err != nil {
			return nil, err
		}
		references = append(references, cr...)
	}

	return references, nil
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
					fmt.Sprintf("unable to decode disabled expression: %s", expdiags.Error()),
				)
		}

		err := types.SetDisabled(r, isDisabled)
		if err != nil {
			return isDisabled, errors.NewParserErrorFromResource(
				r,
				fmt.Sprintf("failed to set disabled state: %s", err),
			)
		}
	}

	return isDisabled, nil
}

// walk builds a DAG from the state and walks it with the given callback
// This is the core parsing logic that processes resources in dependency order
// and calls the provider lifecycle for each resource
func (p *Parser) walk(currentState, previousState *state.State, functions map[string]function.Function) []error {
	// Build the DAG using currentState (implements ResourceProvider)
	d, err := DoYouLikeDags(currentState, false)
	if err != nil {
		return []error{err}
	}

	// Reduce the graph nodes to unique instances
	d.TransitiveReduction()

	// Validate the dependency graph is ok
	err = d.Validate()
	if err != nil {
		return []error{fmt.Errorf("unable to validate dependency graph: %w", err)}
	}

	// Define the walker callback that will be called for every node in the graph
	w := dag.Walker{}

	// For now, previousState is nil as we need to implement state loading
	// TODO: Load previousState from StateStore when implementing state persistence
	var previousParsed *parsed = nil

	w.Callback = walkCallback(p.parsedResources, previousParsed, currentState, p.providerResolver, &p.options, functions)
	w.Reverse = false

	// Update the dag and process the nodes
	log.SetOutput(io.Discard)

	errs := []error{}
	w.Update(d)
	diags := w.Wait()
	if diags.HasErrors() {
		errs = append(errs, diags.Err().(errwrap.Wrapper).WrappedErrors()...)
		return errs
	}

	return nil
}

// createBuiltinResource creates built-in resource types (local, output, variable, module)
func (p *Parser) createBuiltinResource(resourceType, resourceName string) (any, error) {
	builtinTypes := resources.DefaultResources()
	return builtinTypes.CreateResource(resourceType, resourceName)
}

// getFunctions returns all HCL functions (custom + builtins)
func (p *Parser) getFunctions() map[string]function.Function {
	// Start with default built-in functions
	funcs := functions.GetDefaultFunctions("")

	// Override with custom functions from parser options
	for name, fn := range p.customFunctions {
		funcs[name] = fn
	}

	return funcs
}
