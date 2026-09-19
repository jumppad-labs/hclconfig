package parser

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/registry"
	statemocks "github.com/jumppad-labs/xcl/state/mocks"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Positions reported by reference validation are block granular. They point at
// the resource block that holds the reference rather than at the expression
// within it, which is why the assertions below name the line of the block.

func TestValidateReferencesReturnsConfigErrorWhenResourceIsNotDefined(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/undefined_resource_reference")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "container.xcl"), pe.Filename)
	require.Equal(t, 5, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "resource 'resource.container.consul' refers to 'resource.network.nosuch.meta.name', which is not defined anywhere in the configuration", pe.Message)
}

func TestValidateReferencesReturnsConfigErrorWhenVariableIsNotDefined(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/undefined_variable_reference")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "container.xcl"), pe.Filename)
	require.Equal(t, 1, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "resource 'resource.container.consul' refers to 'variable.missing_var', which is not defined anywhere in the configuration", pe.Message)
}

func TestValidateReferencesReturnsConfigErrorWhenOutputIsNotDefined(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/undefined_output_reference")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "output.xcl"), pe.Filename)
	require.Equal(t, 5, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "resource 'output.network_name' refers to 'output.no_such_output', which is not defined anywhere in the configuration", pe.Message)
}

func TestValidateReferencesReturnsConfigErrorWhenModuleIsNotDefined(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/undefined_module_reference")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "output.xcl"), pe.Filename)
	require.Equal(t, 5, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "resource 'output.module_thing' refers to 'module.nosuch.output.thing', which is not defined anywhere in the configuration", pe.Message)
}

func TestValidateReferencesReportsEveryUndefinedReferenceAcrossFiles(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/undefined_references_multi_file")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// the directory holds three separate broken references spread over two
	// files. All three must be reported together, an author must not be made to
	// fix one and rerun to discover the next. They are reported in sorted
	// resource id order, which is why output.bad comes before the containers.
	require.Len(t, ce.Errors, 3)

	first := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "b.xcl"), first.Filename)
	require.Equal(t, 9, first.Line)
	require.Equal(t, 1, first.Column)
	require.Equal(t, "resource 'output.bad' refers to 'resource.container.ghost.meta.name', which is not defined anywhere in the configuration", first.Message)

	second := ce.Errors[1].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "a.xcl"), second.Filename)
	require.Equal(t, 5, second.Line)
	require.Equal(t, 1, second.Column)
	require.Equal(t, "resource 'resource.container.first' refers to 'resource.network.nosuch.meta.name', which is not defined anywhere in the configuration", second.Message)

	third := ce.Errors[2].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "b.xcl"), third.Filename)
	require.Equal(t, 1, third.Line)
	require.Equal(t, 1, third.Column)
	require.Equal(t, "resource 'resource.container.second' refers to 'variable.missing_var', which is not defined anywhere in the configuration", third.Message)
}

func TestValidateReferencesAcceptsReferenceToResourceDeclaredInAnotherFile(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/resolving_cross_file_reference")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// container.xcl refers to a network declared in network.xcl. A reference
	// that crosses files is ordinary and must be accepted, otherwise an
	// implementation that reported everything as missing would look correct.
	_, err := p.Apply(dir)
	require.NoError(t, err)
}

func TestValidateReferencesAcceptsSimpleFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/simple/container.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestValidateReferencesAcceptsModulesFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestValidateReferencesAcceptsInterpolationFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/interpolation/interpolation.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestValidateReferencesAcceptsCyclicalPassFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/cyclical/pass")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

// Module outputs are ordinary output blocks that parsing has already placed in
// the working set under the module's instance name. Resolving one therefore
// needs nothing from the module's own evaluation context, which is what the
// tests below pin down.

func TestResolveReferenceFindsOutputDeclaredByModule(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, _, err := p.parseAndValidate(f)
	require.NoError(t, err)

	resolved := p.resolveReference("module.consul_1.output.container_resources_cpu", "")
	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1.output.container_resources_cpu", resolved.key)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceFindsMapOutputDeclaredByModule(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, _, err := p.parseAndValidate(f)
	require.NoError(t, err)

	resolved := p.resolveReference("module.consul_1.output.combined_map", "")
	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1.output.combined_map", resolved.key)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceDoesNotFindOutputTheModuleDoesNotDeclare(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, _, err := p.parseAndValidate(f)
	require.NoError(t, err)

	resolved := p.resolveReference("module.consul_1.output.nosuch_output", "")
	require.False(t, resolved.found)
	require.Empty(t, resolved.key)
	require.Nil(t, resolved.target)
}

func TestValidateReferencesReturnsConfigErrorWhenModuleDoesNotDeclareOutput(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/undeclared_module_output")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// the module itself is obtainable, so parsing succeeds and reference
	// validation is reached. The output the reference names is not one the
	// module declares, so it must be reported and named.
	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "main.xcl"), pe.Filename)
	require.Equal(t, 5, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "resource 'output.module_thing' refers to 'module.consul.output.nosuch_output', which is not defined anywhere in the configuration", pe.Message)
}

func TestParseReturnsConfigErrorWhenModuleContentsCannotBeObtained(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/unobtainable_module_references")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	// the source is joined onto the directory of the file that declared the
	// module, so the reported source is that absolute path rather than the
	// relative spelling written in the configuration.
	missingDir := filepath.Join(filepath.Dir(dir), "no_such_module_directory")

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "main.xcl"), pe.Filename)
	require.Equal(t, 1, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "unable to obtain contents for module 'missing' source '"+missingDir+"': lstat "+missingDir+": no such file or directory", pe.Message)
}

func TestParseDoesNotReportReferencesIntoModuleWhoseContentsCannotBeObtained(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/unobtainable_module_references")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// the configuration holds three separate references into the module whose
	// contents could not be obtained. None of the outputs they name can exist
	// in the working set, yet the author is told only the one thing that is
	// actually wrong: the module could not be read. Reporting the module
	// failure during parsing is what makes this hold - the parse gate returns
	// before reference validation runs at all.
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Contains(t, pe.Message, "unable to obtain contents for module 'missing'")
	require.NotContains(t, pe.Message, "not defined anywhere in the configuration")

	for _, e := range ce.Errors {
		require.NotContains(t, e.(*errors.ParserError).Message, "not defined anywhere in the configuration")
	}
}

func TestParseReturnsConfigErrorWhenModuleSourceIsNotOnTheLocalFilesystem(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/remote_module_source")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	// a module source is only ever resolved as a directory relative to the
	// file that declared it. A source that names somewhere other than the
	// local filesystem is joined on all the same, resolves to nothing, and is
	// reported as contents that could not be obtained.
	joinedSource := filepath.Join(dir, "github.com/jumppad-labs/example")
	firstMissingSegment := filepath.Join(dir, "github.com")

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "main.xcl"), pe.Filename)
	require.Equal(t, 1, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "unable to obtain contents for module 'remote' source '"+joinedSource+"': lstat "+firstMissingSegment+": no such file or directory", pe.Message)
}

// Property validation is stage three, and it runs only once structure and
// references are known to be sound. Like reference validation, the positions it
// reports are block granular: they name the resource block holding the
// reference rather than the expression within it.
//
// The tests below are paired deliberately. A checker that rejected everything
// would satisfy every negative test here while breaking real configuration, so
// the fixtures that must keep parsing matter at least as much as the ones that
// must fail.

func TestValidatePropertiesReturnsConfigErrorWhenPropertyDoesNotExist(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/bad_property_after_selection")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// the reference selects a volume by position and then names `destnation`,
	// a misspelling of a property those volumes do have. The resource and the
	// reference are both sound, so only the property is wrong.
	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "container.xcl"), pe.Filename)
	require.Equal(t, 10, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Equal(t, "reference 'resource.container.consul.volume[0].destnation' names property 'destnation', which does not exist", pe.Message)
}

func TestValidatePropertiesReportsEveryPropertyProblemAcrossFiles(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/multiple_property_problems")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// three separate property problems spread over two files. All three are
	// reported together, in sorted resource id order, so an author is not made
	// to fix one and rerun to discover the next.
	require.Len(t, ce.Errors, 3)

	first := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "a.xcl"), first.Filename)
	require.Equal(t, 10, first.Line)
	require.Equal(t, 1, first.Column)
	require.Equal(t, "reference 'resource.container.first.volume[0].destnation' names property 'destnation', which does not exist", first.Message)

	second := ce.Errors[1].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "b.xcl"), second.Filename)
	require.Equal(t, 9, second.Line)
	require.Equal(t, 1, second.Column)
	require.Equal(t, "reference 'resource.container.second.meta.nam' names property 'nam', which does not exist", second.Message)

	third := ce.Errors[2].(*errors.ParserError)
	require.Equal(t, filepath.Join(dir, "b.xcl"), third.Filename)
	require.Equal(t, 13, third.Line)
	require.Equal(t, 1, third.Column)
	require.Equal(t, "reference 'resource.container.second.resources.nosuch' names property 'nosuch', which does not exist", third.Message)
}

func TestValidatePropertiesAcceptsSelectionsFollowedByRealProperties(t *testing.T) {
	dir, pathErr := filepath.Abs("../test_fixtures/config/valid_property_paths")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// this configuration names a map key and then a property of the member it
	// picks, a map key and then a property nested beneath that member, a
	// position within a list and then a property of that member, and a key
	// within a map whose members are scalars. Every one of them is ordinary
	// configuration and must be accepted.
	_, err := p.Apply(dir)
	require.NoError(t, err)
}

// The fixtures below already existed and already worked. They are the guard
// against an over eager checker: each of them uses a shape that property
// validation could plausibly misread, and each must keep parsing cleanly.

func TestValidatePropertiesAcceptsFunctionsDefaultFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/functions/default.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// the single best regression case in the repository. Its dns attribute
	// reads `created_network_map != null ? values(...).*.meta.name : []`, which
	// combines a conditional, a function call, a splat and a nested property in
	// one expression. A checker that mishandled any one of those would break it.
	//
	// This fixture is asserted through parseAndValidate rather than Apply
	// because validation is what this stage owns, and validation is what is
	// pinned here. TestApplyProcessesDefaultFunctionsWithFile covers Apply.
	_, _, err := p.parseAndValidate(f)
	require.NoError(t, err)

	require.Empty(t, p.validate())
}

func TestValidatePropertiesAcceptsSimpleFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/simple/container.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestValidatePropertiesAcceptsInterpolationFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/interpolation/interpolation.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// this fixture reaches a volume by position as `volume.0.source` and every
	// volume at once as `volume.*.destination`, both followed by properties
	// those volumes really have.
	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestValidatePropertiesAcceptsModulesFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// forty one resources, and among them
	// `module.consul_1.output.combined_map.name`. A reference to an output
	// names the value that output holds rather than a field of the declaration,
	// so checking `combined_map` against the Output struct would reject this
	// whole fixture.
	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestValidatePropertiesAcceptsCyclicalPassFixture(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/cyclical/pass")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

// Computed fields are owned by the provider. The fixtures below set them in
// configuration, which validation must reject before any provider is reached.

func TestValidateRejectsConfiguredComputedField(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/computed_set/top.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, testPlugin := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, f, pe.Filename)
	require.Contains(t, pe.Message, "resource.network.main")
	require.Contains(t, pe.Message, "provider_id")
	require.Equal(t, "resource 'resource.network.main' sets computed field 'provider_id', computed fields are set by the provider and cannot be configured", pe.Message)

	// validation stops the apply before the provider receives any call
	require.Empty(t, testPlugin.GetCalls())
	require.Empty(t, testPlugin.GetCreatedResources())
}

func TestValidateRejectsConfiguredNestedComputedField(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/computed_set/nested.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, testPlugin := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Contains(t, pe.Message, "resource.container.app")
	require.Contains(t, pe.Message, "network[1].assigned_address")
	require.Equal(t, "resource 'resource.container.app' sets computed field 'network[1].assigned_address', computed fields are set by the provider and cannot be configured", pe.Message)

	require.Empty(t, testPlugin.GetCalls())
}

func TestValidateReportsConfiguredComputedFieldAtAttributePosition(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/computed_set/nested.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	err := p.Validate(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	// assigned_address is written on line 14 of the fixture, inside the second
	// network block, indented by four spaces
	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, f, pe.Filename)
	require.Equal(t, 14, pe.Line)
	require.Equal(t, 5, pe.Column)
}

func TestValidateAcceptsUnsetComputedField(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/computed_set/unset.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

// badComputed is a resource type whose computed field is not optional, which no
// configuration could satisfy since users can not set a computed field
type badComputed struct {
	types.ResourceBase `xcl:",remain"`

	Secret string `xcl:"secret,computed" json:"secret"`
}

// badComputedProvider is a provider for badComputed that does nothing
type badComputedProvider struct {
	plugins.DefaultChanged[*badComputed]
}

func (p *badComputedProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *badComputedProvider) Create(ctx context.Context, resource *badComputed) (*badComputed, error) {
	return resource, nil
}

func (p *badComputedProvider) Destroy(ctx context.Context, resource *badComputed, force bool) error {
	return nil
}

func (p *badComputedProvider) Read(ctx context.Context, old *badComputed, resource *badComputed) (*badComputed, error) {
	return resource, nil
}

func (p *badComputedProvider) Update(ctx context.Context, resource *badComputed) (*badComputed, error) {
	return resource, nil
}

func (p *badComputedProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// badComputedPlugin registers the badComputed resource type
type badComputedPlugin struct {
	plugins.PluginBase
}

func (p *badComputedPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"bad_computed",
		&badComputed{},
		&badComputedProvider{},
	)
}

func TestValidateRejectsNonOptionalComputedField(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/computed_set/non_optional.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	ms := &statemocks.MockStateStore{}
	ms.On("Exists").Return(false)
	ms.On("Load").Return(nil, nil)
	ms.On("Save", mock.Anything).Return(nil)

	o := testOptions(t)
	o.StateStore = ms
	o.PluginRegistry = registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := o.PluginRegistry.RegisterPlugin(&badComputedPlugin{})
	require.NoError(t, err)

	p, _ := setupParser(t, o)

	err = p.Validate(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	// the problem is with the type rather than the configuration, so it is
	// reported at the resource block
	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, f, pe.Filename)
	require.Equal(t, 1, pe.Line)
	require.Contains(t, pe.Message, "is computed and must be optional")
	require.Contains(t, pe.Message, "secret")
	require.Equal(t, "resource 'resource.bad_computed.x' field 'secret' is computed and must be optional", pe.Message)
}
