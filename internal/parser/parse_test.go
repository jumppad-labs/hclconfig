package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/parser/mocks"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/logger"
	pluginmocks "github.com/jumppad-labs/xcl/plugins/mocks"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	statemocks "github.com/jumppad-labs/xcl/state/mocks"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/jumppad-labs/xcl/internal/cty"
)

// findResource is a test helper that finds and converts a resource to the given type.
// It uses JSON marshal/unmarshal to convert the anonymous struct to the named type.
func findResource[T any](t *testing.T, s *state.State, path string) *T {
	t.Helper()
	r, err := s.FindResource(path)
	require.NoError(t, err)

	result := new(T)
	err = schema.UnmarshalUntyped(r, result)
	require.NoError(t, err)

	return result
}

// testOptions returns DefaultOptions with a test logger and the module cache
// in the test's temp directory, so that tests never write to the working copy
// or the users home folder
func testOptions(t *testing.T) *ParserOptions {
	o := DefaultOptions()
	o.Logger = logger.NewTestLogger(t)
	o.ModuleCache = filepath.Join(t.TempDir(), ConfigDirectory, "cache")

	return o
}

func setupParser(t *testing.T, options ...*ParserOptions) (*Parser, *TestPlugin) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	var o *ParserOptions

	if len(options) > 0 {
		o = options[0]
	} else {
		ms := &statemocks.MockStateStore{}
		ms.On("Exists").Return(false)
		ms.On("Load").Return(nil, nil)
		ms.On("Save", mock.Anything).Return(nil)

		o = testOptions(t)
		o.StateStore = ms
	}

	// Always use TestLogger for all parser tests (override default StdOutLogger)
	o.Logger = logger.NewTestLogger(t)

	// Create a plugin registry for the parser (Config normally owns this, but for standalone parser tests we create one)
	if o.PluginRegistry == nil {
		o.PluginRegistry = registry.NewPluginRegistry(o.Logger)
	}

	p := NewParser(o)

	// Create and register the test plugin
	testPlugin := &TestPlugin{}
	err := o.PluginRegistry.RegisterPlugin(testPlugin)
	if err != nil {
		panic("Failed to register test plugin: " + err.Error())
	}

	return p, testPlugin
}

func TestNewParserWithOptions(t *testing.T) {
	options := ParserOptions{
		Variables:      map[string]string{"foo": "bar"},
		VariablesFiles: []string{"./myfile.txt"},
		ModuleCache:    "./modules",
		Logger:         logger.NewTestLogger(t),
	}

	p := NewParser(&options)
	require.NotNil(t, p)

	require.Equal(t, p.options.Variables["foo"], "bar")
	require.Equal(t, p.options.VariablesFiles[0], "./myfile.txt")
	require.Equal(t, p.options.ModuleCache, "./modules")
}

func TestParseFileProcessesResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple/container.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	cont := findResource[structs.Container](t, c, "resource.container.consul")

	vr, err := c.FindResource("variable.cpu_resources")
	require.NoError(t, err)
	require.NotNil(t, vr)

	require.Equal(t, "resource.container.consul", cont.Meta.ID)
	require.Equal(t, "consul", cont.Meta.Name)
	require.Equal(t, absoluteFolderPath, cont.Meta.File)

	require.Equal(t, "consul", cont.Command[0], "consul")
	require.Equal(t, "10.6.0.200", cont.Networks[0].IPAddress)
	require.Equal(t, 1024, cont.Resources.CPU)

	base := findResource[structs.Container](t, c, "resource.container.base")
	require.NotNil(t, base)
}

func TestParseFileSetsLinks(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple/container.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// parser should replace any resource links with an empty value and return a list
	// of links and the field paths where they were originally set
	// this enables us to build a graph of objects and later set these fields to the correct
	// reference values
	require.Len(t, cont.Meta.Links, 10)

	require.Contains(t, cont.Meta.Links, "resource.network.onprem.meta.name")
	require.Contains(t, cont.Meta.Links, "resource.container.base.dns")
	require.Contains(t, cont.Meta.Links, "resource.container.base.resources.cpu_pin")
	require.Contains(t, cont.Meta.Links, "resource.container.base.resources.memory")
	require.Contains(t, cont.Meta.Links, "resource.container.base.resources.user")
	require.Contains(t, cont.Meta.Links, "resource.container.base.network[0].id")
	require.Contains(t, cont.Meta.Links, "resource.container.base.network[1].name")
	require.Contains(t, cont.Meta.Links, "resource.template.consul_config.destination")
	require.Contains(t, cont.Meta.Links, "resource.template.consul_config.meta.name")
	require.Contains(t, cont.Meta.Links, "variable.cpu_resources")
}

func TestParseResolvesArrayReferences(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple/container.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	out := findResource[resources.Output](t, c, "output.ip_address_1")
	require.Equal(t, "10.6.0.200", out.Value)

	// check variable has been interpolated
	out = findResource[resources.Output](t, c, "output.ip_address_2")
	require.Equal(t, "10.7.0.201", out.Value)

	out = findResource[resources.Output](t, c, "output.ip_addresses")
	require.Equal(t, "10.6.0.200", out.Value.([]any)[0].(string))
	require.Equal(t, "10.7.0.201", out.Value.([]any)[1].(string))
	require.Equal(t, float64(12), out.Value.([]any)[2].(float64))
}

func TestParseSetsDefaultValues(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/defaults/container.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.default")

	// check default values have been set
	require.Equal(t, "hello world", cont.Default)
}

func TestLoadsVariableFilesInOptionsOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	ms := &statemocks.MockStateStore{}
	ms.On("Load").Return(nil, nil)
	ms.On("Save", mock.Anything).Return(nil)
	ms.On("Exists").Return(false)

	o := testOptions(t)
	o.StateStore = ms
	o.VariablesFiles = []string{filepath.Join(absoluteFolderPath, "vars", "override.vars")}

	p, _ := setupParser(t, o)

	c, err := p.Apply(filepath.Join(absoluteFolderPath, "container.xcl"))
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// check variable has been interpolated using the override value
	require.Equal(t, 4096, cont.Resources.CPU)
}

func TestLoadsVariablesInEnvVarOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	p, _ := setupParser(t)

	os.Setenv("HCL_VAR_cpu_resources", "1000")

	t.Cleanup(func() {
		os.Unsetenv("HCL_VAR_cpu_resources")
	})

	c, err := p.Apply(filepath.Join(absoluteFolderPath, "container.xcl"))
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// check variable has been interpolated using the override value
	require.Equal(t, 1000, cont.Resources.CPU)
}

func TestLoadsVariableFilesInDirectoryOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// check variable has been interpolated using the override value
	require.Equal(t, 1024, cont.Resources.CPU)
}

func TestLoadsVariablesFilesOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// check variable has been interpolated using the override value
	require.Equal(t, 1024, cont.Resources.CPU)
}

func TestResourceReferencesInExpressionsAreEvaluated(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/interpolation/interpolation.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	require.Len(t, c.GetResources(), 10)

	_ = findResource[structs.Container](t, c, "resource.container.consul")

	out := findResource[resources.Output](t, c, "output.splat")
	require.Equal(t, "/cache", out.Value.([]any)[0])
	require.Equal(t, "/cache2", out.Value.([]any)[1])

	out = findResource[resources.Output](t, c, "output.splat_with_null")
	// Since created_network is not populated in the config, this should return an empty array
	require.Equal(t, []any{}, out.Value)

	out = findResource[resources.Output](t, c, "output.function")
	require.Equal(t, float64(2), out.Value)

	out = findResource[resources.Output](t, c, "output.binary")
	require.Equal(t, false, out.Value)

	out = findResource[resources.Output](t, c, "output.condition")
	require.Equal(t, "/cache", out.Value)

	out = findResource[resources.Output](t, c, "output.template")
	require.Equal(t, "abc/2", out.Value)

	out = findResource[resources.Output](t, c, "output.index")
	require.Equal(t, "images.volume.shipyard.run", out.Value)

	out = findResource[resources.Output](t, c, "output.index_interpolated")
	require.Equal(t, "root/images.volume.shipyard.run", out.Value)

}

func TestResourceReferencesInExpressionStringsAreEvaluated(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/interpolation/string.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	con := findResource[structs.Container](t, c, "resource.container.container4")
	require.Equal(t, "8500", con.Env["port_string"])
}

func TestParseModuleCreatesResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	require.Len(t, c.GetResources(), 41)

	// check resource has been created
	cont := findResource[structs.Container](t, c, "module.consul_1.resource.container.consul")

	// check interpolation value
	require.Equal(t, "onprem", cont.Networks[0].Name)

	// check resource has been created
	cont = findResource[structs.Container](t, c, "module.consul_2.resource.container.consul")

	require.Equal(t, "onprem", cont.Networks[0].Name)

	// check resource has been created
	cont = findResource[structs.Container](t, c, "module.consul_3.resource.container.consul")

	// check interpolation value
	require.Equal(t, "onprem", cont.Networks[0].Name)

}

func TestParseModuleDoesNotCacheLocalFiles(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)
	require.NotNil(t, c)

	// local module sources are read directly from disk on every parse and
	// must never be materialized into the module cache
	require.NoDirExists(t, filepath.Join(p.options.ModuleCache, "single"))
}

func TestParseModuleCreatesOutputs(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	require.Len(t, c.GetResources(), 41)

	out := findResource[resources.Output](t, c, "output.module1_container_resources_cpu")

	// check output value from module is equal to the module variable
	// which is set as an interpolated value of the container base
	require.Equal(t, float64(4096), out.Value)

	out = findResource[resources.Output](t, c, "output.module2_container_resources_cpu")

	// check output value from module is equal to the module variable
	// which is set as the variable for the config
	require.Equal(t, float64(512), out.Value)

	out = findResource[resources.Output](t, c, "output.module3_container_resources_cpu")

	// check the output variable is set to the default value for the module
	require.Equal(t, float64(2048), out.Value)

	out = findResource[resources.Output](t, c, "output.module1_from_list_1")

	out2 := findResource[resources.Output](t, c, "output.module1_from_list_2")

	// check an element can be obtained from a list of values
	// returned from a output
	require.Equal(t, float64(0), out.Value)
	require.Equal(t, float64(4096), out2.Value)

	// check an element can be obtained from a map of values
	// returned from a output
	out = findResource[resources.Output](t, c, "output.module1_from_map_1")

	out2 = findResource[resources.Output](t, c, "output.module1_from_map_2")

	// check element can be obtained from a map of values
	// returned in the output
	require.Equal(t, "consul", out.Value)
	require.Equal(t, float64(4096), out2.Value)

	out = findResource[resources.Output](t, c, "output.object")

	// check element can be obtained from a map of values
	// returned in the output
	meta := out.Value.(map[string]any)["meta"].(map[string]any)
	require.Equal(t, "base", meta["name"])
}

func TestDoesNotLoadsVariablesFilesFromInsideModules(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/var_files.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	cont := findResource[structs.Container](t, c, "module.consul_1.resource.container.consul")
	require.Equal(t, 2048, cont.Resources.CPU)
}

func TestModuleDisabledCanBeOverriden(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// test disabled overrides are set
	cont := findResource[structs.Container](t, c, "module.consul_2.resource.container.sidecar")

	// check disabled has been interpolated
	require.False(t, cont.Disabled)

	// check that the module resources callbacks are called
	// TODO: re-enable when lifecycle is implemented
	// require.Contains(t, calls, "module.consul_2.resource.container.sidecar")

	// test disabled is maintainerd
	cont = findResource[structs.Container](t, c, "module.consul_1.resource.container.sidecar")

	// check disabled has been interpolated
	require.True(t, cont.Disabled)

	// check that the module resources callbacks are called
	// TODO: re-enable when lifecycle is implemented
	// require.NotContains(t, calls, "module.consul_1.resource.container.sidecar")
}

func TestParseContainerWithNoNameReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/invalid/no_name.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	_, err = p.Apply(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithNoTypeReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/invalid/no_type.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	_, err = p.Apply(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithNoTLDReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/invalid/no_resource.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	_, err = p.Apply(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseDoesNotProcessDisabledResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/disabled/disabled.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)
	require.Equal(t, 5, c.ResourceCount())

	r, err := c.FindResource("resource.container.disabled_value")
	require.NoError(t, err)
	disabled, err := types.GetDisabled(r)
	require.NoError(t, err)
	require.True(t, disabled)

	r, err = c.FindResource("resource.container.disabled_variable")
	require.NoError(t, err)
	disabled, err = types.GetDisabled(r)
	require.NoError(t, err)
	require.True(t, disabled)

	// should have been called for the variable and network (not disabled)
	// TODO: re-enable when lifecycle is implemented
	// require.Len(t, calls, 2)
}

func TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/disabled/module.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("module.disabled.resource.container.enabled")
	require.NoError(t, err)

	disabled, err := types.GetDisabled(r)
	require.NoError(t, err)
	require.True(t, disabled)

	r, err = c.FindResource("module.disabled.sub.resource.container.enabled")
	require.NoError(t, err)

	disabled, err = types.GetDisabled(r)
	require.NoError(t, err)
	require.True(t, disabled)

	// should only called for the containing module and variables
	// TODO: re-enable when lifecycle is implemented
	// require.Len(t, calls, 3)
}

func TestGetNameAndIndexReturnsCorrectDetails(t *testing.T) {
	path := []string{"resource", "foo", "bar"}
	n, i, rp, err := getNameAndIndex(path)
	require.NoError(t, err)
	require.Equal(t, "resource", n)
	require.Equal(t, -1, i)
	require.Equal(t, []string{"foo", "bar"}, rp)

	path = []string{"resource", "foo[0]", "bar"}
	n, i, rp, err = getNameAndIndex(path)
	require.NoError(t, err)
	require.Equal(t, "resource", n)
	require.Equal(t, -1, i)
	require.Equal(t, []string{"foo[0]", "bar"}, rp)

	path = []string{"foo[0]", "bar"}
	n, i, rp, err = getNameAndIndex(path)
	require.NoError(t, err)
	require.Equal(t, "foo", n)
	require.Equal(t, 0, i)
	require.Equal(t, []string{"bar"}, rp)

	path = []string{"foo[nic]", "bar"}
	_, _, _, err = getNameAndIndex(path)
	require.Error(t, err)

	path = []string{"foo", "0", "bar"}
	n, i, rp, err = getNameAndIndex(path)
	require.NoError(t, err)
	require.Equal(t, "foo", n)
	require.Equal(t, 0, i)
	require.Equal(t, []string{"bar"}, rp)

	path = []string{"bar[0]"}
	n, i, rp, err = getNameAndIndex(path)
	require.NoError(t, err)
	require.Equal(t, "bar", n)
	require.Equal(t, 0, i)
	require.Equal(t, []string{}, rp)
}

func TestSetContextVariableFromPath(t *testing.T) {
	ctx := &hcl.EvalContext{}
	ctx.Variables = map[string]cty.Value{"resource": cty.ObjectVal(map[string]cty.Value{})}

	err := setContextVariableFromPath(ctx, "resource.foo.bar", cty.BoolVal(true))
	require.NoError(t, err)

	err = setContextVariableFromPath(ctx, "resource.foo.biz", cty.StringVal("Hello World"))
	require.NoError(t, err)

	err = setContextVariableFromPath(ctx, "resource.foo.bear.grr", cty.StringVal("Grrrr"))

	require.NoError(t, err)

	err = setContextVariableFromPath(ctx, "resource.poo", cty.StringVal("Meh"))
	require.NoError(t, err)

	require.True(t, ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["bar"].True())
	require.Equal(t, "Hello World", ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["biz"].AsString())
	require.Equal(t, "Grrrr", ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["bear"].AsValueMap()["grr"].AsString())
	require.Equal(t, "Meh", ctx.Variables["resource"].AsValueMap()["poo"].AsString())
}

func TestSetContextVariableFromPathWithEndingIndex(t *testing.T) {
	ctx := &hcl.EvalContext{}
	ctx.Variables = map[string]cty.Value{"resource": cty.ObjectVal(map[string]cty.Value{})}

	err := setContextVariableFromPath(ctx, "resource.foo.bar", cty.ListVal([]cty.Value{cty.BoolVal(false), cty.BoolVal(false)}))
	require.NoError(t, err)

	err = setContextVariableFromPath(ctx, "resource.foo.bar[0]", cty.BoolVal(true))
	require.NoError(t, err)

	err = setContextVariableFromPath(ctx, "resource.foo.bar[1]", cty.BoolVal(false))

	require.NoError(t, err)

	require.True(t, ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["bar"].AsValueSlice()[0].True())
	require.False(t, ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["bar"].AsValueSlice()[1].True())
}

func TestSetContextVariableFromPathWithIndex(t *testing.T) {
	ctx := &hcl.EvalContext{}
	ctx.Variables = map[string]cty.Value{"resource": cty.ObjectVal(map[string]cty.Value{})}

	err := setContextVariableFromPath(ctx, "resource.foo[0].bar", cty.BoolVal(true))
	require.NoError(t, err)

	err = setContextVariableFromPath(ctx, "resource.foo.1.biz", cty.StringVal("Hello World"))

	require.NoError(t, err)

	fmt.Println(ctx.Variables["resource"].AsValueMap()["foo"].Type().FriendlyName())
	require.True(t, ctx.Variables["resource"].AsValueMap()["foo"].AsValueSlice()[0].AsValueMap()["bar"].True())
	require.Equal(t, "Hello World", ctx.Variables["resource"].AsValueMap()["foo"].AsValueSlice()[1].AsValueMap()["biz"].AsString())
}

func TestParserProcessesResourcesInCorrectOrder(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	ms := &statemocks.MockStateStore{}
	ms.On("Load").Return(nil, nil)
	ms.On("Save", mock.Anything).Return(nil)
	ms.On("Exists").Return(false)

	// Every resource in this fixture is new, so the lifecycle walk only ever calls Create.
	// A single mock adapter that echoes its input stands in for every resource type/provider -
	// this test only cares that a provider was invoked in the correct order, not what it does.
	adapter := pluginmocks.NewMockProviderAdapter(t)
	adapter.EXPECT().Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, entityData []byte) ([]byte, error) { return entityData, nil })

	resolver := mocks.NewMockProviderResolver(t)
	resolver.EXPECT().GetProviderForResource(mock.Anything).Return(adapter)

	o := testOptions(t)
	o.StateStore = ms
	o.ProviderResolver = resolver

	calls := []string{}
	var callsMu sync.Mutex // the walker fires events from parallel goroutines

	o.OnParserEvent = func(event ParserEvent) {
		if event.Operation == "create" && event.Phase == "success" {
			callsMu.Lock()
			defer callsMu.Unlock()
			calls = append(calls, event.ResourceID)
		}
	}

	p, _ := setupParser(t, o)

	_, err = p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// check the order, should be ...
	// resource.container.base
	// -- module.consul_1
	// -- -- module.consul_1.resource.network.onprem
	// -- -- -- module.consul_1.resource.container.consul
	// -- -- -- -- module.consul_1.resource.output.container_name
	// -- -- -- -- module.consul_1.resource.output.container_resources_cpu
	// -- -- -- -- -- resource.output.module_1_container_resources_cpu
	// -- -- -- -- -- -- resource.module.consul_3
	// -- -- -- -- -- -- -- module.consul_3.resource.network.onprem
	// -- -- -- -- -- -- -- -- module.consul_3.resource.container.consul
	// -- -- -- -- -- -- -- -- -- module.consul_3.resource.output.container_name
	// -- -- -- -- -- -- -- -- -- module.consul_3.resource.output.container_resources_cpu
	// -- -- -- -- -- -- -- -- -- -- resource.output.module_1_container_resources_cpu
	// module.consul_2
	// -- module.consul_2.resource.network.onprem
	// -- -- module.consul_2.resource.container.consul
	// -- -- -- module.consul_2.resource.output.container_name
	// -- -- -- module.consul_2.resource.output.container_resources_cpu
	// -- -- -- -- resource.output.module_2_container_resources_cpu

	// module1 depends on an attribute of resource.container.base, all resources in module1 should only
	// be processed after container.base has been created
	requireBefore(t, "resource.container.base", "module.consul_1.resource.network.onprem", calls)

	// resource.network.onprem in module.consul_2 should be created after the top level module is created
	requireBefore(t, "resource.module.consul_2", "module.consul_2.resource.network.onprem", calls)

	// resource.container.consul in module consul_2 depends on resource.network.onprem in module2 it should always
	// be created after the network
	requireBefore(t, "module.consul_2.resource.network.onprem", "module.consul_2.resource.container.consul", calls)

	// the output module_1_container_resources_cpu depends on an output defined in module consul_1, it should always be created
	// after all resources in module consul_1
	requireBefore(t, "module.consul_1.resource.container.consul", "output.module1_container_resources_cpu", calls)

	// the module should always be created before its resources
	requireBefore(t, "module.consul_1", "module.consul_1.resource.container.consul", calls)

	// the output module_2_container_resources_cpu depends on an output defined in module consul_2, it should always be created
	// after all resources in module consul_2
	requireBefore(t, "module.consul_2.resource.container.consul", "output.module2_container_resources_cpu", calls)

	// the module consul_3 has a hard coded dependency on module_1, it should only be created after all
	// resources in module_1 have been created
	requireBefore(t, "module.consul_1.resource.container.consul", "module.consul_3.resource.container.consul", calls)
	requireBefore(t, "module.consul_1.resource.cotnainer.consul", "module.consul_1.output.container_resources_cpu", calls)
}

func TestParserErrorsOnPluginCreateError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, tp := setupParser(t)

	// ensure an error is returned when creating a resource
	tp.SetCreateError("resource.container.base", fmt.Errorf("test error"))

	_, err = p.Apply(absoluteFolderPath)
	require.Error(t, err)

	cr := tp.GetCreatedResources()

	// Verify the error occurred and the resource was tracked
	require.Len(t, cr, 4)
	require.Contains(t, cr, "resource.container.base")
	require.Contains(t, cr, "module.consul_2.resource.container.consul")
	require.NotContains(t, cr, "module.consul_1.resource.container.consul")
}

func TestParserRejectsInvalidResourceName(t *testing.T) {
	// should reject names starting with a number
	err := validateResourceName("0")
	require.Error(t, err)

	// should reject names containing invalid characters
	err = validateResourceName("my resource")
	require.Error(t, err)

	err = validateResourceName("my*resource")
	require.Error(t, err)

	// should reject reserved names
	err = validateResourceName("variable")
	require.Error(t, err)

	err = validateResourceName("output")
	require.Error(t, err)

	err = validateResourceName("resource")
	require.Error(t, err)

	err = validateResourceName("module")
	require.Error(t, err)

	// should be valid
	err = validateResourceName("0232module")
	require.NoError(t, err)

	err = validateResourceName("0232m_od-ule")
	require.NoError(t, err)

	err = validateResourceName("my_Module")
	require.NoError(t, err)
}

func TestParserCyclicalReferenceReturnsError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/cyclical/fail/cyclical.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.Error(t, err)

	require.ErrorContains(t, err, "'resource.container.one' depends on 'resource.network.two'")
}

func TestParserNoCyclicalReferenceReturns(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/cyclical/pass/cyclical.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.NoError(t, err)
}

func TestParseDirectoryReturnsConfigErrorWhenParseDirectoryFails(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/invalid")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 3)
}

func TestParseDirectoryReturnsConfigErrorWhenResourceProcessError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// of the files in this directory only bad_format.xcl is malformed at parse
	// time, so that is the problem that must be reported and it must be
	// reported as a failure, located in that file
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, filepath.Join(f, "bad_format.xcl"), pe.Filename)
	require.Equal(t, 7, pe.Line)
	require.Equal(t, 30, pe.Column)
	require.Contains(t, pe.Message, "unable to parse file")
}

func TestParseFileReturnsConfigErrorWhenParseDirectoryFails(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/invalid/no_name.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseFileReturnsConfigErrorWhenResourceBadlyFormed(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/bad_format.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, f, pe.Filename)
	require.Equal(t, 7, pe.Line)
	require.Equal(t, 30, pe.Column)
}

func TestParseFileReturnsConfigErrorWhenFunctionError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/function_error.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
	require.IsType(t, &errors.ParserError{}, ce.Errors[0])
}

func TestParseFileReturnsConfigErrorWhenResourceInterpolationError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/bad_interpolation.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	// the fixture refers to 'resource.network.test.nam', a misspelling of the
	// 'name' property. This was previously reported as an advisory warning and
	// the configuration was treated as usable; it is now a failure naming the
	// property that does not exist.
	require.NotEmpty(t, ce.Errors)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Contains(t, pe.Message, "nam")
	require.Contains(t, pe.Message, "does not exist")
}

func TestParseFileReturnsConfigErrorWhenInvalidFileFails(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/invalid/notexist.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParserEventCallback(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	// Track all events
	var events []ParserEvent
	var eventsMu sync.Mutex // the walker fires events from parallel goroutines

	// Setup parser with event callback
	options := testOptions(t)
	options.Logger = logger.NewTestLogger(t)
	options.OnParserEvent = func(event ParserEvent) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	p, _ := setupParser(t, options)

	// Parse the file - this should trigger create events
	_, err = p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// Verify events were fired
	require.NotEmpty(t, events, "Expected parser events to be fired")

	// Check that we have start and success events for any operation
	var startEvents []ParserEvent
	var successEvents []ParserEvent

	for _, event := range events {
		// parse events are not provider operations, they have tests of their own
		if event.Operation == "parse" {
			continue
		}

		if event.Phase == "start" {
			startEvents = append(startEvents, event)
		}
		if event.Phase == "success" {
			successEvents = append(successEvents, event)
		}
	}

	require.NotEmpty(t, startEvents, "Expected operation start events")
	require.NotEmpty(t, successEvents, "Expected operation success events")

	// Verify event structure for success events
	for _, event := range successEvents {
		require.Contains(t, []string{"create", "read", "changed", "update", "destroy"}, event.Operation, "Expected valid operation type")
		require.Equal(t, "success", event.Phase)
		require.Contains(t, event.ResourceType, ".", "Expected resource type to contain a dot")
		require.NotEmpty(t, event.ResourceID, "Expected resource ID to be set")

		// Builtin types (variables, outputs, locals, modules, root) have 0 duration
		if strings.Contains(event.ResourceType, "variable.") ||
			strings.Contains(event.ResourceType, "output.") ||
			strings.Contains(event.ResourceType, "module.") ||
			strings.Contains(event.ResourceType, "root.") {
			require.Equal(t, time.Duration(0), event.Duration, "Expected 0 duration for builtin types")
		} else {
			require.Greater(t, event.Duration, time.Duration(0), "Expected duration to be greater than 0 for provider operations")
		}

		require.NoError(t, event.Error, "Expected no error for success events")

		// Builtin types don't have data
		if !strings.Contains(event.ResourceType, "variable.") &&
			!strings.Contains(event.ResourceType, "output.") &&
			!strings.Contains(event.ResourceType, "module.") &&
			!strings.Contains(event.ResourceType, "root.") {
			require.NotEmpty(t, event.Data, "Expected data to be set for provider operations")
		}
	}
}

func TestParserCreateEventErrorCallback(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	require.NoError(t, err)

	var events []ParserEvent
	var eventsMu sync.Mutex // the walker fires events from parallel goroutines

	options := testOptions(t)
	options.OnParserEvent = func(event ParserEvent) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	p, tp := setupParser(t, options)

	// nothing exists in state, so the resource is created
	tp.SetCreateError("resource.container.base", fmt.Errorf("test create error"))

	_, err = p.Apply(absoluteFolderPath)
	require.Error(t, err)

	// create event with start phase is fired before the plugins Create method is called
	requireEvent(t, events, "create", "start", "resource.container.base")

	// create event with error phase is fired when an error is returned during the
	// plugin Create method
	event := requireEvent(t, events, "create", "error", "resource.container.base")
	require.ErrorContains(t, event.Error, "test create error")
	require.Greater(t, event.Duration, time.Duration(0))
	require.NotEmpty(t, event.Data)
}

func TestParserReadEventErrorCallback(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	require.NoError(t, err)

	// first apply creates the resources that the second apply finds in state
	firstStore := &statemocks.MockStateStore{}
	firstStore.On("Exists").Return(false)

	firstOptions := testOptions(t)
	firstOptions.StateStore = firstStore

	firstParser, _ := setupParser(t, firstOptions)

	previousState, err := firstParser.Apply(absoluteFolderPath)
	require.NoError(t, err)

	var events []ParserEvent
	var eventsMu sync.Mutex // the walker fires events from parallel goroutines

	secondStore := &statemocks.MockStateStore{}
	secondStore.On("Exists").Return(true)
	secondStore.On("Load").Return(previousState, nil)

	secondOptions := testOptions(t)
	secondOptions.StateStore = secondStore
	secondOptions.OnParserEvent = func(event ParserEvent) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	secondParser, tp := setupParser(t, secondOptions)

	// the resource exists in state, so it is read rather than created
	tp.SetReadError("resource.container.base", fmt.Errorf("test read error"))

	_, err = secondParser.Apply(absoluteFolderPath)
	require.Error(t, err)

	requireEvent(t, events, "read", "start", "resource.container.base")

	event := requireEvent(t, events, "read", "error", "resource.container.base")
	require.ErrorContains(t, event.Error, "test read error")
	require.Greater(t, event.Duration, time.Duration(0))
	require.NotEmpty(t, event.Data)
}

func TestParserEventForVariablesOutputsLocals(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	// Track all events
	var events []ParserEvent
	var eventsMu sync.Mutex // the walker fires events from parallel goroutines

	// Setup parser with event callback
	options := testOptions(t)
	options.Logger = logger.NewTestLogger(t)
	options.OnParserEvent = func(event ParserEvent) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	p, _ := setupParser(t, options)

	// Parse the file - this should trigger events for variables, outputs, and locals
	_, err = p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	// Verify events were fired
	require.NotEmpty(t, events, "Expected parser events to be fired")

	// Check for variable, output, and module events
	var variableEvents []ParserEvent
	var outputEvents []ParserEvent
	var moduleEvents []ParserEvent

	for _, event := range events {
		if event.Operation == "create" && event.Phase == "success" {
			if strings.Contains(event.ResourceType, "variable.") {
				variableEvents = append(variableEvents, event)
			}
			if strings.Contains(event.ResourceType, "output.") {
				outputEvents = append(outputEvents, event)
			}
			if strings.Contains(event.ResourceType, "module.") {
				moduleEvents = append(moduleEvents, event)
			}
		}
	}

	require.NotEmpty(t, variableEvents, "Expected variable events to be fired")
	require.NotEmpty(t, outputEvents, "Expected output events to be fired")
	require.NotEmpty(t, moduleEvents, "Expected module events to be fired")

	// Verify event structure for variables and outputs
	for _, event := range variableEvents {
		require.Equal(t, "create", event.Operation)
		require.Equal(t, "success", event.Phase)
		require.Contains(t, event.ResourceType, "variable.", "Expected variable resource type")
		require.NotEmpty(t, event.ResourceID, "Expected resource ID to be set")
		require.Equal(t, time.Duration(0), event.Duration, "Expected 0 duration for variables")
		require.NoError(t, event.Error, "Expected no error for success events")
	}

	for _, event := range outputEvents {
		require.Equal(t, "create", event.Operation)
		require.Equal(t, "success", event.Phase)
		require.Contains(t, event.ResourceType, "output.", "Expected output resource type")
		require.NotEmpty(t, event.ResourceID, "Expected resource ID to be set")
		require.Equal(t, time.Duration(0), event.Duration, "Expected 0 duration for outputs")
		require.NoError(t, event.Error, "Expected no error for success events")
	}
}

// requireEvent fails the test unless events contains an event with the given
// operation, phase and resource ID, and returns the first one that matches.
func requireEvent(t *testing.T, events []ParserEvent, operation, phase, resourceID string) ParserEvent {
	t.Helper()

	fired := []string{}
	for _, event := range events {
		if event.Operation == operation && event.Phase == phase && event.ResourceID == resourceID {
			return event
		}

		fired = append(fired, fmt.Sprintf("%s %s %s", event.Operation, event.Phase, event.ResourceID))
	}

	require.FailNow(t, fmt.Sprintf("expected %s %s event for %s. events: %v", operation, phase, resourceID, fired))
	return ParserEvent{}
}

func requireBefore(t *testing.T, first, second string, list []string) {
	// get the positions
	pos1 := -1
	pos2 := -1

	for i, el := range list {
		if first == el {
			pos1 = i
		}

		if second == el {
			pos2 = i
		}
	}

	require.Greater(t, pos2, pos1, fmt.Sprintf("expected %s to be created before %s. calls: %v", first, second, list))
}

func TestParseFileReportsEveryMalformedBlockInFile(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/multiple_errors/two_blocks_missing_names.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// the fixture contains two separate malformations, a nameless resource block
	// starting at line 3 and another starting at line 7. Both must be reported,
	// parsing must not stop at the first.
	require.Len(t, ce.Errors, 2)

	first := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, f, first.Filename)
	require.Equal(t, 3, first.Line)
	require.Equal(t, 1, first.Column)
	require.Contains(t, first.Message, "has no name")

	second := ce.Errors[1].(*errors.ParserError)
	require.Equal(t, f, second.Filename)
	require.Equal(t, 7, second.Line)
	require.Equal(t, 1, second.Column)
	require.Contains(t, second.Message, "has no name")
}

func TestParseFileReportsEveryDiagnosticForSingleMalformation(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/multiple_errors/unterminated_string.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// the unterminated quoted string on line 4 makes the HCL parser raise three
	// related complaints, every one of them must be surfaced rather than only
	// the first.
	require.Len(t, ce.Errors, 3)

	for _, e := range ce.Errors {
		pe := e.(*errors.ParserError)
		require.Equal(t, f, pe.Filename)
		require.Greater(t, pe.Line, 0)
		require.Greater(t, pe.Column, 0)
	}

	// the unterminated string itself starts on line 4 of the fixture
	require.Equal(t, 4, ce.Errors[0].(*errors.ParserError).Line)
}

func TestParseFileReportsMalformedBlockAsErrorNotWarning(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/multiple_errors/unterminated_string.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// a file that is not well formed is a failure, and every problem in it is
	// reported as a parser error
	require.NotEmpty(t, ce.Errors)

	for _, e := range ce.Errors {
		require.IsType(t, &errors.ParserError{}, e)
	}
}

func TestParseFileReportsFileAndPositionForMalformedBlock(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/bad_format.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)

	// the missing opening brace is on line 7 of the fixture, the parser reports
	// it at the point the block body should have started
	require.Equal(t, f, pe.Filename)
	require.Equal(t, 7, pe.Line)
	require.Equal(t, 30, pe.Column)
}

func TestParseFileToleratesUninterpolatableValue(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/bad_interpolation.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// this file is well formed, its only fault is a value that cannot yet be
	// interpolated. The parse stage must not report it as a malformed file, so
	// none of the reported problems may be a parse diagnostic.
	for _, e := range ce.Errors {
		pe := e.(*errors.ParserError)
		require.NotContains(t, pe.Message, "unable to parse file")
	}
}

func TestParseCreatesNothingWhenConfigurationIsRejected(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/multiple_errors/two_blocks_missing_names.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, testPlugin := setupParser(t)

	// the provider lifecycle always runs, so the only thing that can stop the
	// walk is the gate that runs before resources reach state
	s, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)
	require.Nil(t, s)

	// nothing in a rejected configuration may reach a provider
	require.Empty(t, testPlugin.GetCreatedResources())
	require.Empty(t, testPlugin.GetUpdatedResources())
	require.Empty(t, testPlugin.GetDestroyedResources())
}

func TestParseCreatesNothingWhenOneFileInDirectoryIsRejected(t *testing.T) {
	dir := t.TempDir()

	// a wholly valid file, which would be created were files judged one at a time
	writeErr := os.WriteFile(filepath.Join(dir, "network.xcl"), []byte(`
resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}
`), 0644)
	require.NoError(t, writeErr)

	// a sibling whose block is missing its name label
	writeErr = os.WriteFile(filepath.Join(dir, "container.xcl"), []byte(`
resource "container" {
  command = ["consul", "agent", "-dev"]
}
`), 0644)
	require.NoError(t, writeErr)

	p, testPlugin := setupParser(t)

	s, err := p.Apply(dir)
	require.IsType(t, &errors.ConfigError{}, err)
	require.Nil(t, s)

	// the valid network in the sibling file must not have been created, the
	// configuration is judged as one unit
	require.Empty(t, testPlugin.GetCreatedResources())
}

func TestParseResourceReturnsConfigErrorWhenTypeIsNotRegistered(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/unregistered_type/unknown.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)

	// only the 'nosuchtype' block is unknown, the network beside it is a
	// registered type and must not be reported
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)

	// the report must name both the resource and the type that could not be
	// checked, not the literal block keyword 'resource'
	require.Equal(
		t,
		"unable to create resource 'example' of type 'nosuchtype': resource type nosuchtype not found in any registered plugin",
		pe.Message,
	)
}

func TestParseResourceWithUnregisteredTypeReportsWhereItAppears(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/unregistered_type/unknown.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)

	// the offending block starts on line 5 of the fixture, at column 1
	require.Equal(t, f, pe.Filename)
	require.Equal(t, 5, pe.Line)
	require.Equal(t, 1, pe.Column)
}

func TestParseModuleReturnsConfigErrorWhenModuleSourcesItself(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/self_including_module/self.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// the guard must make this terminate; without it the parse recurses until
	// the stack is exhausted, so fail rather than hang the suite
	type parseResult struct {
		err error
	}

	done := make(chan parseResult, 1)
	go func() {
		_, err := p.Apply(f)
		done <- parseResult{err: err}
	}()

	var result parseResult
	select {
	case result = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("parsing a self-including module did not terminate within 30s, the recursion guard has regressed")
	}

	require.IsType(t, &errors.ConfigError{}, result.err)

	ce := result.err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)
	require.Contains(t, pe.Message, "module 'inner' source")
	require.Contains(t, pe.Message, "includes itself")

	// the failure is reported against the module block that re-entered, which
	// is the one declared on line 1 of the module's own source file
	require.Equal(t, 1, pe.Line)
	require.Equal(t, 1, pe.Column)
}

func TestParseModuleReturnsConfigErrorWhenModulesIncludeEachOther(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/mutual_modules/a/a.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	// a includes b and b includes a; indirect recursion must terminate too
	type parseResult struct {
		err error
	}

	done := make(chan parseResult, 1)
	go func() {
		_, err := p.Apply(f)
		done <- parseResult{err: err}
	}()

	var result parseResult
	select {
	case result = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("parsing mutually including modules did not terminate within 30s, the recursion guard has regressed")
	}

	require.IsType(t, &errors.ConfigError{}, result.err)

	ce := result.err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)

	// parsing starts at a/a.xcl, which enters b, whose module block re-enters
	// a, whose module block then sources b a second time. The re-entry is
	// detected there, on the 'b_module' block declared in directory a.
	require.Contains(t, pe.Message, "module 'b_module' source")
	require.Contains(t, pe.Message, "includes itself")
}

func TestParseModuleReturnsConfigErrorWhenSourceDoesNotExist(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/missing_module_source/missing.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Apply(f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	pe := ce.Errors[0].(*errors.ParserError)

	// reported against the module block itself, naming the module whose
	// contents could not be obtained
	require.Equal(t, f, pe.Filename)
	require.Equal(t, 1, pe.Line)
	require.Equal(t, 1, pe.Column)
	require.Contains(t, pe.Message, "unable to obtain contents for module 'gone' source")
}

func TestParseModuleParsesSameSourceUsedTwiceAsSiblings(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/module_reused_twice/reuse.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	c, err := p.Apply(f)
	require.NoError(t, err)
	require.NotNil(t, c)

	// two module blocks share one source but are siblings, not nested, so the
	// recursion guard must not reject the second one. Each module instance and
	// its single network resource must be present: 2 modules + 2 networks.
	require.Len(t, c.GetResources(), 4)

	first := findResource[structs.Network](t, c, "module.first.resource.network.leafnet")
	require.Equal(t, "10.7.0.0/16", first.Subnet)

	second := findResource[structs.Network](t, c, "module.second.resource.network.leafnet")
	require.Equal(t, "10.7.0.0/16", second.Subnet)
}
