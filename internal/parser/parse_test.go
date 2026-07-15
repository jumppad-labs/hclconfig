package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/state/mocks"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
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
		ms := &mocks.MockStateStore{}
		ms.On("Exists").Return(false)
		ms.On("Load").Return(nil, nil)
		ms.On("Save", mock.Anything).Return(nil)

		o = DefaultOptions()
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.default")

	// check default values have been set
	require.Equal(t, "hello world", cont.Default)
}

func TestLoadsVariableFilesInOptionsOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(nil, nil)
	ms.On("Save", mock.Anything).Return(nil)
	ms.On("Exists").Return(false)

	o := DefaultOptions()
	o.StateStore = ms
	o.VariablesFiles = []string{filepath.Join(absoluteFolderPath, "vars", "override.vars")}

	p, _ := setupParser(t, o)

	c, err := p.Parse(false, filepath.Join(absoluteFolderPath, "container.xcl"))
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

	c, err := p.Parse(false, filepath.Join(absoluteFolderPath, "container.xcl"))
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// check variable has been interpolated using the override value
	require.Equal(t, 1000, cont.Resources.CPU)
}

func TestLoadsVariableFilesInDirectoryOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	p, _ := setupParser(t)

	c, err := p.Parse(false, absoluteFolderPath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.consul")

	// check variable has been interpolated using the override value
	require.Equal(t, 1024, cont.Resources.CPU)
}

func TestLoadsVariablesFilesOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple")
	require.NoError(t, err)

	p, _ := setupParser(t)

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	_, err = p.Parse(false, absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithNoTypeReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/invalid/no_type.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	_, err = p.Parse(false, absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithNoTLDReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/invalid/no_resource.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	_, err = p.Parse(false, absoluteFolderPath)
	require.Error(t, err)
}

func TestParseDoesNotProcessDisabledResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/disabled/disabled.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := setupParser(t)

	c, err := p.Parse(false, absoluteFolderPath)
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

	c, err := p.Parse(false, absoluteFolderPath)
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

	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(nil, nil)
	ms.On("Save", mock.Anything).Return(nil)

	o := DefaultOptions()
	o.StateStore = ms

	calls := []string{}

	o.OnParserEvent = func(event ParserEvent) {
		if event.Operation == "create" && event.Phase == "success" {
			calls = append(calls, event.ResourceID)
		}
	}

	p, _ := setupParser(t, o)

	_, err = p.Parse(false, absoluteFolderPath)
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

func TestParserStopsParseOnCreateError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	p, tp := setupParser(t)

	// ensure an error is returned when creating a resource
	tp.SetCreateError("resource.container.base", fmt.Errorf("test error"))

	_, err = p.Parse(false, absoluteFolderPath)
	require.Error(t, err)

	cr := tp.GetCreatedResources()

	// Verify the error occurred and the resource was tracked
	require.Len(t, cr, 4)
	require.Contains(t, cr, "resource.container.base")
	require.Contains(t, cr, "module.consul_2.resource.container.consul")
	require.NotContains(t, cr, "module.consul_1.resource.container.consul")
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

	_, err := p.Parse(false, f)
	require.Error(t, err)

	require.ErrorContains(t, err, "'resource.container.one' depends on 'resource.network.two'")
}

func TestParserNoCyclicalReferenceReturns(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/cyclical/pass/cyclical.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
	require.NoError(t, err)
}

func TestParseDirectoryReturnsConfigErrorWhenParseDirectoryFails(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/invalid")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseDirectoryReturnsConfigErrorWhenResourceProcessError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseFileReturnsConfigErrorWhenParseDirectoryFails(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/invalid/no_name.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
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

	_, err := p.Parse(false, f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	require.True(t, ce.ContainsErrors())

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, pe.Level, errors.ParserErrorLevelError)
}

func TestParseFileReturnsConfigErrorWhenFunctionError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/function_error.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	require.True(t, ce.ContainsErrors())

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, pe.Level, errors.ParserErrorLevelError)
}

func TestParseFileReturnsConfigErrorWhenResourceInterpolationError(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/process_error/bad_interpolation.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
	require.IsType(t, &errors.ConfigError{}, err)

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	require.False(t, ce.ContainsErrors())

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, pe.Level, errors.ParserErrorLevelWarning)
}

func TestParseFileReturnsConfigErrorWhenInvalidFileFails(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/config/invalid/notexist.xcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p, _ := setupParser(t)

	_, err := p.Parse(false, f)
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

	// Setup parser with event callback
	options := DefaultOptions()
	options.Logger = logger.NewTestLogger(t)
	options.OnParserEvent = func(event ParserEvent) {
		events = append(events, event)
	}

	p, _ := setupParser(t, options)

	// Parse the file - this should trigger create events
	_, err = p.Parse(false, absoluteFolderPath)
	require.NoError(t, err)

	// Verify events were fired
	require.NotEmpty(t, events, "Expected parser events to be fired")

	// Check that we have start and success events for any operation
	var startEvents []ParserEvent
	var successEvents []ParserEvent

	for _, event := range events {
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
		require.Contains(t, []string{"create", "refresh", "changed", "update", "destroy"}, event.Operation, "Expected valid operation type")
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

func TestParserEventErrorCallback(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	// Track all events
	var events []ParserEvent

	// Setup parser with event callback
	options := DefaultOptions()
	options.Logger = logger.NewTestLogger(t)
	options.OnParserEvent = func(event ParserEvent) {
		events = append(events, event)
	}

	p, tp := setupParser(t, options)

	// Configure the plugin to return an error for refresh operations (since resources exist in state)
	tp.SetRefreshError("resource.container.base", fmt.Errorf("test refresh error"))

	// Parse the file - this should trigger error events
	_, err = p.Parse(false, absoluteFolderPath)
	require.Error(t, err, "Expected parsing to fail due to refresh error")

	// Verify events were fired
	require.NotEmpty(t, events, "Expected parser events to be fired")

	// Check that we have start and error events for the operation
	var startEvents []ParserEvent
	var errorEvents []ParserEvent

	for _, event := range events {
		if event.Phase == "start" {
			startEvents = append(startEvents, event)
		}
		if event.Phase == "error" {
			errorEvents = append(errorEvents, event)
		}
	}

	require.NotEmpty(t, startEvents, "Expected operation start events")
	require.NotEmpty(t, errorEvents, "Expected operation error events")

	// Verify error event structure
	for _, event := range errorEvents {
		require.Contains(t, []string{"create", "refresh", "changed", "update", "destroy"}, event.Operation, "Expected valid operation type")
		require.Equal(t, "error", event.Phase)
		require.Contains(t, event.ResourceType, ".", "Expected resource type to contain a dot")
		require.NotEmpty(t, event.ResourceID, "Expected resource ID to be set")
		require.Greater(t, event.Duration, time.Duration(0), "Expected duration to be greater than 0")
		require.Error(t, event.Error, "Expected error for error events")
		require.Contains(t, event.Error.Error(), "test refresh error", "Expected error message to contain test error")
		require.NotEmpty(t, event.Data, "Expected data to be set")
	}
}

func TestParserEventForVariablesOutputsLocals(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/modules/modules.xcl")
	if err != nil {
		t.Fatal(err)
	}

	// Track all events
	var events []ParserEvent

	// Setup parser with event callback
	options := DefaultOptions()
	options.Logger = logger.NewTestLogger(t)
	options.OnParserEvent = func(event ParserEvent) {
		events = append(events, event)
	}

	p, _ := setupParser(t, options)

	// Parse the file - this should trigger events for variables, outputs, and locals
	_, err = p.Parse(false, absoluteFolderPath)
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

func TestDestroyLifecycle(t *testing.T) {
	// Setup parser with file state store
	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(nil, nil)
	ms.On("Save", mock.Anything).Return(nil)

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p, testPlugin := setupParser(t, o)

	// First parse: create resources
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/config/simple/container.xcl")
	require.NoError(t, err)

	config1, err := p.Parse(false, absoluteFolderPath)
	require.NoError(t, err)
	require.NotNil(t, config1)

	// Verify resources were created
	createdResources := testPlugin.GetCreatedResources()
	require.Contains(t, createdResources, "resource.container.consul")
	require.Contains(t, createdResources, "resource.container.base")

	// Mock state store to return the created config as previous state
	ms.ExpectedCalls = nil // Clear previous expectations
	ms.On("Load").Return(config1, nil)
	ms.On("Save", mock.Anything).Return(nil)

	// Create a smaller config (remove some resources)
	p2, testPlugin2 := setupParser(t, o)

	// Parse a config with fewer resources (to trigger destroy)
	absoluteFolderPath2, err := filepath.Abs("../test_fixtures/config/defaults/container.xcl")
	require.NoError(t, err)

	config2, err := p2.Parse(false, absoluteFolderPath2)
	require.NoError(t, err)
	require.NotNil(t, config2)

	// Verify destroy operations were called for removed resources
	destroyedResources := testPlugin2.GetDestroyedResources()

	// Resources from config1 that are not in config2 should be destroyed
	// This will depend on what's actually in the test fixtures
	require.NotEmpty(t, destroyedResources, "Expected some resources to be destroyed")
}

// TODO: The tests below test Destroy functionality that has moved from Parser to Config.
// They need to be rewritten to test Config.Destroy() instead of Parser.Destroy()
/*
func TestDestroyDependencyValidation(t *testing.T) {
	// Test that destroy validation prevents destroying resources that others depend on
	p, _ := setupParser(t)

	// Test validateDestroyDependencies directly with empty slices
	toDestroy := []any{} // Resources to destroy
	remaining := []any{} // Resources that remain

	errors := p.validateDestroyDependencies(toDestroy, remaining)
	require.Empty(t, errors, "Expected no errors when no dependencies exist")
}

func TestDestroyWithNoState(t *testing.T) {
	// Test Destroy when there's no existing state
	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(nil, nil)

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p := NewParser(o)

	config, err := p.Destroy()
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.Resources)

	ms.AssertNotCalled(t, "Save")
}

func TestDestroyWithEmptyState(t *testing.T) {
	// Test Destroy when state exists but has no resources
	existingState := NewConfig()

	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(existingState, nil)

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p := NewParser(o)

	config, err := p.Destroy()
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.Resources)

	ms.AssertNotCalled(t, "Save")
}

func TestDestroyWithResources(t *testing.T) {
	// Test Destroy when state has resources
	existingState := NewConfig()

	// Create test resources with proper metadata
	container1 := &structs.Container{
		ContainerBase: structs.ContainerBase{
			ResourceBase: types.ResourceBase{
				Meta: types.Meta{
					Name:   "test1",
					Type:   "container",
					ID:     "resource.container.test1",
					Status: "created",
				},
			},
		},
	}

	container2 := &structs.Container{
		ContainerBase: structs.ContainerBase{
			ResourceBase: types.ResourceBase{
				Meta: types.Meta{
					Name:   "test2",
					Type:   "container",
					ID:     "resource.container.test2",
					Status: "created",
				},
			},
		},
	}

	existingState.Resources = append(existingState.Resources, container1, container2)

	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(existingState, nil)
	ms.On("Save", mock.Anything).Return(nil)

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p, testPlugin := setupParser(t, o)

	config, err := p.Destroy()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Check that resources were destroyed
	destroyedResources := testPlugin.GetDestroyedResources()
	require.Len(t, destroyedResources, 2)

	// Check that destroyed resources were removed from config
	require.Empty(t, config.Resources)

	// Check that state was saved
	ms.AssertCalled(t, "Save", mock.Anything)
}

func TestDestroyWithFailedDestroy(t *testing.T) {
	// Test Destroy when destroy operation fails
	existingState := NewConfig()

	// Create a test resource
	container := &structs.Container{
		ContainerBase: structs.ContainerBase{
			ResourceBase: types.ResourceBase{
				Meta: types.Meta{
					Name:   "test1",
					Type:   "container",
					ID:     "resource.container.test1",
					Status: "created",
				},
			},
		},
	}

	existingState.Resources = append(existingState.Resources, container)

	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(existingState, nil)
	ms.On("Save", mock.Anything).Return(nil)

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p, testPlugin := setupParser(t, o)

	// Configure the test plugin to fail on destroy for this specific resource
	testPlugin.SetDestroyError("resource.container.test1", fmt.Errorf("destroy failed"))

	config, err := p.Destroy()
	require.Error(t, err)
	require.Contains(t, err.Error(), "destroy phase failed")
	require.NotNil(t, config)

	// Check that failed resource was not removed from config
	require.Len(t, config.Resources, 1)

	// State should still be saved even on error
	ms.AssertCalled(t, "Save", mock.Anything)
}

func TestDestroyWithInvalidStateType(t *testing.T) {
	// Test Destroy when state has wrong type
	invalidState := "not a config"

	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(invalidState, nil)

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p := NewParser(o)

	config, err := p.Destroy()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid state type")
	require.Nil(t, config)
}

func TestDestroyWithStateLoadError(t *testing.T) {
	// Test Destroy when state load fails
	ms := &mocks.MockStateStore{}
	ms.On("Load").Return(nil, fmt.Errorf("failed to load state"))

	o := DefaultOptions()
	o.StateStore = ms
	o.Logger = logger.NewTestLogger(t)

	p := NewParser(o)

	config, err := p.Destroy()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load state")
	require.Nil(t, config)
}
*/
