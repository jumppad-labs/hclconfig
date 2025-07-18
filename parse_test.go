package hclconfig

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/hclconfig/errors"
	"github.com/jumppad-labs/hclconfig/resources"
	"github.com/jumppad-labs/hclconfig/test_fixtures/embedded"
	"github.com/jumppad-labs/hclconfig/test_fixtures/structs"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func setupParser(t *testing.T, options ...*ParserOptions) *Parser {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	o := DefaultOptions()
	if len(options) > 0 {
		o = options[0]
	}

	p := NewParser(o)
	p.RegisterType("container", &structs.Container{})
	p.RegisterType("network", &structs.Network{})
	p.RegisterType("template", &structs.Template{})
	p.RegisterType(structs.TypeParseError, &structs.ParseError{})

	return p
}

func TestNewParserWithOptions(t *testing.T) {
	options := ParserOptions{
		Variables:      map[string]string{"foo": "bar"},
		VariablesFiles: []string{"./myfile.txt"},
		ModuleCache:    "./modules",
	}

	p := NewParser(&options)
	require.NotNil(t, p)

	require.Equal(t, p.options.Variables["foo"], "bar")
	require.Equal(t, p.options.VariablesFiles[0], "./myfile.txt")
	require.Equal(t, p.options.ModuleCache, "./modules")
}

func TestParseFileProcessesResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)

	v, err := c.FindResource("variable.cpu_resources")
	require.NoError(t, err)
	require.NotNil(t, v)

	cont := r.(*structs.Container)

	require.Equal(t, "resource.container.consul", cont.Metadata().ID)
	require.Equal(t, "consul", cont.Metadata().Name)
	require.Equal(t, absoluteFolderPath, cont.Metadata().File)

	require.Equal(t, "consul", cont.Command[0], "consul")
	require.Equal(t, "10.6.0.200", cont.Networks[0].IPAddress)
	require.Equal(t, 2048, cont.Resources.CPU)

	r, err = c.FindResource("resource.container.base")
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestParseFileCallsParseFunction(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)

	cont := r.(*structs.Container)
	require.Equal(t, "something", cont.Meta.Properties["status"])
}

func TestParseFileSetsLinks(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)

	// parser should replace any resource links with an empty value and return a list
	// of links and the field paths where they were originally set
	// this enables us to build a graph of objects and later set these fields to the correct
	// reference values
	cont := r.(*structs.Container)
	require.Len(t, cont.Meta.Links, 9)

	require.Contains(t, cont.Meta.Links, "resource.network.onprem.meta.name")
	require.Contains(t, cont.Meta.Links, "resource.container.base.dns")
	require.Contains(t, cont.Meta.Links, "resource.container.base.resources.cpu_pin")
	require.Contains(t, cont.Meta.Links, "resource.container.base.resources.memory")
	require.Contains(t, cont.Meta.Links, "resource.container.base.resources.user")
	require.Contains(t, cont.Meta.Links, "resource.container.base.network[0].id")
	require.Contains(t, cont.Meta.Links, "resource.container.base.network[1].name")
	require.Contains(t, cont.Meta.Links, "resource.template.consul_config.destination")
	require.Contains(t, cont.Meta.Links, "resource.template.consul_config.meta.name")
}

func TestParseResolvesArrayReferences(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("output.ip_address_1")
	require.NoError(t, err)
	require.NotNil(t, r)

	out := r.(*resources.Output)
	require.Equal(t, "10.6.0.200", out.Value)

	// check variable has been interpolated
	r, err = c.FindResource("output.ip_address_2")
	require.NoError(t, err)
	require.NotNil(t, r)

	out = r.(*resources.Output)
	require.Equal(t, "10.7.0.201", out.Value)

	r, err = c.FindResource("output.ip_addresses")
	require.NoError(t, err)
	require.NotNil(t, r)

	out = r.(*resources.Output)
	require.Equal(t, "10.6.0.200", out.Value.([]any)[0].(string))
	require.Equal(t, "10.7.0.201", out.Value.([]any)[1].(string))
	require.Equal(t, float64(12), out.Value.([]any)[2].(float64))
}

func TestParseSetsDefaultValues(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/defaults/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.default")
	require.NoError(t, err)
	require.NotNil(t, r)

	// check default values have been set
	cont := r.(*structs.Container)
	require.Equal(t, "hello world", cont.Default)
}

func TestLoadsVariableFilesInOptionsOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple")
	require.NoError(t, err)

	o := DefaultOptions()
	o.VariablesFiles = []string{filepath.Join(absoluteFolderPath, "vars", "override.vars")}

	p := setupParser(t, o)

	c, err := p.ParseFile(filepath.Join(absoluteFolderPath, "container.hcl"))
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	// check variable has been interpolated using the override value
	cont := r.(*structs.Container)
	require.Equal(t, 4096, cont.Resources.CPU)
}

func TestLoadsVariablesInEnvVarOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple")
	require.NoError(t, err)

	p := setupParser(t)

	os.Setenv("HCL_VAR_cpu_resources", "1000")

	t.Cleanup(func() {
		os.Unsetenv("HCL_VAR_cpu_resources")
	})

	c, err := p.ParseFile(filepath.Join(absoluteFolderPath, "container.hcl"))
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	// check variable has been interpolated using the override value
	cont := r.(*structs.Container)
	require.Equal(t, 1000, cont.Resources.CPU)
}

func TestLoadsVariableFilesInDirectoryOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple")
	require.NoError(t, err)

	p := setupParser(t)

	c, err := p.ParseDirectory(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	// check variable has been interpolated using the override value
	cont := r.(*structs.Container)
	require.Equal(t, 1024, cont.Resources.CPU)
}

func TestLoadsVariablesFilesOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple")
	require.NoError(t, err)

	p := setupParser(t)

	c, err := p.ParseDirectory(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	// check variable has been interpolated using the override value
	cont := r.(*structs.Container)
	require.Equal(t, 1024, cont.Resources.CPU)
}

func TestResourceReferencesInExpressionsAreEvaluated(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/interpolation/interpolation.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	//require.Len(t, c.Resources, 5)

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)
	con := r.(*structs.Container)
	_ = con

	r, err = c.FindResource("output.splat")
	require.NoError(t, err)
	cont := r.(*resources.Output)
	require.Equal(t, "/cache", cont.Value.([]any)[0])
	require.Equal(t, "/cache2", cont.Value.([]any)[1])

	r, err = c.FindResource("output.splat_with_null")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, "test1", cont.Value.([]any)[0])
	require.Equal(t, "test2", cont.Value.([]any)[1])

	r, err = c.FindResource("output.function")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, float64(2), cont.Value)

	r, err = c.FindResource("output.binary")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, false, cont.Value)

	r, err = c.FindResource("output.condition")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, "/cache", cont.Value)

	r, err = c.FindResource("output.template")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, "abc/2", cont.Value)

	r, err = c.FindResource("output.index")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, "images.volume.shipyard.run", cont.Value)

	r, err = c.FindResource("output.index_interpolated")
	require.NoError(t, err)
	cont = r.(*resources.Output)
	require.Equal(t, "root/images.volume.shipyard.run", cont.Value)

}

func TestResourceReferencesInExpressionStringsAreEvaluated(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/interpolation/string.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.container4")
	require.NoError(t, err)
	con := r.(*structs.Container)
	require.Equal(t, "8500", con.Env["port_string"])
}

func TestLocalVariablesCanEvaluateResourceAttributes(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/locals/locals.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	//require.Len(t, c.Resources, 4)
}

func TestParseModuleCreatesResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	require.Len(t, c.Resources, 41)

	// check resource has been created
	cont, err := c.FindResource("module.consul_1.resource.container.consul")
	require.NoError(t, err)

	// check interpolation value
	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)

	// check resource has been created
	cont, err = c.FindResource("module.consul_2.resource.container.consul")
	require.NoError(t, err)

	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)

	// check resource has been created
	cont, err = c.FindResource("module.consul_3.resource.container.consul")
	require.NoError(t, err)

	// check interpolation value
	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)

}

func TestParseModuleDoesNotCacheLocalFiles(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)
	require.NotNil(t, c)

	// the remote module should be cached
	require.DirExists(t, filepath.Join(p.options.ModuleCache, "github.com_jumppad-labs_hclconfig_test_fixtures_single"))

	// the local module should not be cached
	require.NoDirExists(t, filepath.Join(p.options.ModuleCache, "single"))
}

func TestParseModuleCreatesOutputs(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	require.Len(t, c.Resources, 41)

	cont, err := c.FindResource("output.module1_container_resources_cpu")
	require.NoError(t, err)

	// check output value from module is equal to the module variable
	// which is set as an interpolated value of the container base
	require.Equal(t, float64(4096), cont.(*resources.Output).Value)

	cont, err = c.FindResource("output.module2_container_resources_cpu")
	require.NoError(t, err)

	// check output value from module is equal to the module variable
	// which is set as the variable for the config
	require.Equal(t, float64(512), cont.(*resources.Output).Value)

	cont, err = c.FindResource("output.module3_container_resources_cpu")
	require.NoError(t, err)

	// check the output variable is set to the default value for the module
	require.Equal(t, float64(2048), cont.(*resources.Output).Value)

	cont, err = c.FindResource("output.module1_from_list_1")
	require.NoError(t, err)

	cont2, err := c.FindResource("output.module1_from_list_2")
	require.NoError(t, err)

	// check an element can be obtained from a list of values
	// returned from a output
	require.Equal(t, float64(0), cont.(*resources.Output).Value)
	require.Equal(t, float64(4096), cont2.(*resources.Output).Value)

	// check an element can be obtained from a map of values
	// returned from a output
	cont, err = c.FindResource("output.module1_from_map_1")
	require.NoError(t, err)

	cont2, err = c.FindResource("output.module1_from_map_2")
	require.NoError(t, err)

	// check element can be obtained from a map of values
	// returned in the output
	require.Equal(t, "consul", cont.(*resources.Output).Value)
	require.Equal(t, float64(4096), cont2.(*resources.Output).Value)

	cont, err = c.FindResource("output.object")
	require.NoError(t, err)

	// check element can be obtained from a map of values
	// returned in the output
	meta := cont.(*resources.Output).Value.(map[string]any)["meta"].(map[string]any)
	require.Equal(t, "base", meta["name"])
}

func TestDoesNotLoadsVariablesFilesFromInsideModules(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/var_files.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("module.consul_1.resource.container.consul")
	require.NoError(t, err)

	cont := r.(*structs.Container)
	require.Equal(t, 2048, cont.Resources.CPU)
}

func TestModuleDisabledCanBeOverriden(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	callbackMutext := sync.Mutex{}

	calls := []string{}
	o := DefaultOptions()
	o.Callback = func(r types.Resource) error {
		callbackMutext.Lock()
		log.Printf("callback: %s", r.Metadata().ID)
		calls = append(calls, r.Metadata().ID)

		callbackMutext.Unlock()

		return nil
	}

	p := setupParser(t, o)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// test disabled overrides are set
	r, err := c.FindResource("module.consul_2.resource.container.sidecar")
	require.NoError(t, err)

	// check disabled has been interpolated
	cont := r.(*structs.Container)
	require.False(t, cont.Disabled)

	// check that the module resources callbacks are called
	require.Contains(t, calls, "module.consul_2.resource.container.sidecar")

	// test disabled is maintainerd
	r, err = c.FindResource("module.consul_1.resource.container.sidecar")
	require.NoError(t, err)

	// check disabled has been interpolated
	cont = r.(*structs.Container)
	require.True(t, cont.Disabled)

	// check that the module resources callbacks are called
	require.NotContains(t, calls, "module.consul_1.resource.container.sidecar")
}

func TestParseContainerWithNoNameReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/invalid/no_name.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithInvalidAttributeReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/invalid/invalid_attribute.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithNoTypeReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/invalid/no_type.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseContainerWithNoTLDReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/invalid/no_resource.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)
}

func TestResoucesInDisabledModulesRemainDisabled(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/disabled/module.hcl")
	if err != nil {
		t.Fatal(err)
	}

	o := DefaultOptions()
	calls := []string{}
	callSync := sync.Mutex{}
	o.Callback = func(r types.Resource) error {
		callSync.Lock()
		calls = append(calls, r.Metadata().ID)
		callSync.Unlock()

		return nil
	}

	p := setupParser(t, o)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	m, err := c.FindResource("module.disabled")
	require.NoError(t, err)
	require.True(t, m.GetDisabled())

	r, err := c.FindResource("module.disabled.resource.network.onprem")
	require.NoError(t, err)
	require.True(t, r.GetDisabled())
}

func TestParseDoesNotProcessDisabledResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/disabled/disabled.hcl")
	if err != nil {
		t.Fatal(err)
	}

	o := DefaultOptions()
	calls := []string{}
	callSync := sync.Mutex{}
	o.Callback = func(r types.Resource) error {
		callSync.Lock()
		calls = append(calls, r.Metadata().ID)
		callSync.Unlock()

		return nil
	}

	p := setupParser(t, o)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)
	require.Equal(t, 5, c.ResourceCount())

	r, err := c.FindResource("resource.container.disabled_value")
	require.NoError(t, err)
	require.True(t, r.GetDisabled())

	r, err = c.FindResource("resource.container.disabled_variable")
	require.NoError(t, err)
	require.True(t, r.GetDisabled())

	// should have only be called for the variable
	require.Len(t, calls, 1)
}

func TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/disabled/module.hcl")
	if err != nil {
		t.Fatal(err)
	}

	o := DefaultOptions()
	calls := []string{}
	callSync := sync.Mutex{}
	o.Callback = func(r types.Resource) error {
		callSync.Lock()
		calls = append(calls, r.Metadata().ID)
		callSync.Unlock()

		return nil
	}

	p := setupParser(t, o)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("module.disabled.resource.container.enabled")
	require.NoError(t, err)
	require.True(t, r.GetDisabled())

	r, err = c.FindResource("module.disabled.sub.resource.container.enabled")
	require.NoError(t, err)
	require.True(t, r.GetDisabled())

	// should only called for the containing module and variables
	require.Len(t, calls, 5)
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
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	o := DefaultOptions()
	calls := []string{}
	callSync := sync.Mutex{}
	o.Callback = func(r types.Resource) error {
		callSync.Lock()

		calls = append(calls, r.Metadata().ID)

		// add a fake delay
		time.Sleep(10 * time.Millisecond)

		callSync.Unlock()

		return nil
	}

	p := setupParser(t, o)

	_, err = p.ParseFile(absoluteFolderPath)
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

func TestParserStopsParseOnCallbackError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	o := DefaultOptions()
	calls := []string{}
	callSync := sync.Mutex{}
	o.Callback = func(r types.Resource) error {
		callSync.Lock()

		calls = append(calls, resources.FQRN{
			Module:   r.Metadata().Module,
			Resource: r.Metadata().Name,
			Type:     r.Metadata().Type,
		}.String())

		callSync.Unlock()

		if r.Metadata().Name == "base" {
			return fmt.Errorf("container base error")
		}

		return nil
	}

	p := setupParser(t, o)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)

	// only 17 of the resources and variables should be created, none of the descendants of base
	require.GreaterOrEqual(t, len(calls), 16)
	require.LessOrEqual(t, len(calls), 17)
	require.NotContains(t, "resource.module.consul_1", calls)
}

func TestParserDeserializesJSONCorrectly(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	json, err := c.ToJSON()
	require.NoError(t, err)

	conf, err := p.UnmarshalJSON(json)
	require.NoError(t, err)
	require.NotNil(t, conf)

	orig, err := c.FindResource("resource.container.base")
	require.NoError(t, err)

	parsed, err := conf.FindResource("resource.container.base")
	require.NoError(t, err)

	require.Equal(t, orig.Metadata().File, parsed.Metadata().File)
	require.Equal(t, orig.(*structs.Container).Networks[0].Name, parsed.(*structs.Container).Networks[0].Name)
	require.Equal(t, orig.(*structs.Container).Command, parsed.(*structs.Container).Command)
	require.Equal(t, orig.(*structs.Container).Resources.CPUPin, parsed.(*structs.Container).Resources.CPUPin)

	orig, err = c.FindResource("resource.container.consul")
	require.NoError(t, err)

	parsed, err = conf.FindResource("resource.container.consul")
	require.NoError(t, err)

	require.Equal(t, orig.(*structs.Container).Volumes[0].Destination, parsed.(*structs.Container).Volumes[0].Destination)
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

func TestParserGeneratesChecksums(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/simple/container.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	c, err := p.ParseFile(f)
	require.NoError(t, err)

	r1, err := c.FindResource("resource.network.onprem")
	require.NoError(t, err)
	require.NotEmpty(t, r1.Metadata().Checksum.Parsed)

	r2, err := c.FindResource("variable.cpu_resources")
	require.NoError(t, err)
	require.NotEmpty(t, r2.Metadata().Checksum.Parsed)

	r3, err := c.FindResource("output.ip_address_1")
	require.NoError(t, err)
	require.NotEmpty(t, r3.Metadata().Checksum.Parsed)

	// parse a second time, the checksums should be equal
	p = setupParser(t)

	c, err = p.ParseFile(f)
	require.NoError(t, err)

	c1, err := c.FindResource("resource.network.onprem")
	require.NoError(t, err)
	require.Equal(t, r1.Metadata().Checksum.Parsed, c1.Metadata().Checksum.Parsed)

	c2, err := c.FindResource("variable.cpu_resources")
	require.NoError(t, err)
	require.Equal(t, r2.Metadata().Checksum.Parsed, c2.Metadata().Checksum.Parsed)

	c3, err := c.FindResource("output.ip_address_1")
	require.NoError(t, err)
	require.Equal(t, r3.Metadata().Checksum.Parsed, c3.Metadata().Checksum.Parsed)
}

func TestParserCyclicalReferenceReturnsError(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/cyclical/fail/cyclical.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.Error(t, err)

	require.ErrorContains(t, err, "'resource.container.one' depends on 'resource.network.two'")
}

func TestParserNoCyclicalReferenceReturns(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/cyclical/pass/cyclical.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.NoError(t, err)
}

func TestParseDirectoryReturnsConfigErrorWhenParseDirectoryFails(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/invalid")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseDirectory(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseDirectoryReturnsConfigErrorWhenResourceParseError(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/parse_error")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseDirectory(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseDirectoryReturnsConfigErrorWhenResourceProcessError(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/process_error")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseDirectory(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseFileReturnsConfigErrorWhenParseDirectoryFails(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/invalid/no_name.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseFileReturnsConfigErrorWhenResourceParseError(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/parse_error/resource_parse.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseFileReturnsConfigErrorWhenResourceBadlyFormed(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/process_error/bad_format.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	require.True(t, ce.ContainsErrors())

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, pe.Level, errors.ParserErrorLevelError)
}

func TestParseFileReturnsConfigErrorWhenFunctionError(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/process_error/function_error.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	require.True(t, ce.ContainsErrors())

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, pe.Level, errors.ParserErrorLevelError)
}

func TestParseFileReturnsConfigErrorWhenResourceContainsInvalidInterpolation(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/process_error/bad_interpolation.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)

	require.True(t, ce.ContainsErrors())

	pe := ce.Errors[0].(*errors.ParserError)
	require.Equal(t, pe.Level, errors.ParserErrorLevelError)
}

func TestParseFileReturnsConfigErrorWhenInvalidFileFails(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/invalid/notexist.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := setupParser(t)

	_, err := p.ParseFile(f)
	require.IsType(t, err, &errors.ConfigError{})

	ce := err.(*errors.ConfigError)
	require.Len(t, ce.Errors, 1)
}

func TestParseDoesNotOverwiteWithMeta(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/embedded/config.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := NewParser(nil)
	p.RegisterType(embedded.TypeContainer, &embedded.Container{})
	p.RegisterType(embedded.TypeSidecar, &embedded.Sidecar{})

	c, err := p.ParseFile(f)
	require.NoError(t, err)

	r1, err := c.FindResource("resource.container.mine")
	require.NoError(t, err)

	// test that when the meta is set it does not overwrite any
	// existing fields when they have the same name
	cont := r1.(*embedded.Container)
	require.Equal(t, "resource.container.mine", cont.Meta.ID)
	require.Equal(t, "mycontainer", cont.ID)
}

func TestParseHandlesCommonTypes(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/embedded/config.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	p := NewParser(nil)
	p.RegisterType(embedded.TypeContainer, &embedded.Container{})
	p.RegisterType(embedded.TypeSidecar, &embedded.Sidecar{})

	c, err := p.ParseFile(f)
	require.NoError(t, err)

	r1, err := c.FindResource("resource.container.mine")
	require.NoError(t, err)

	cont := r1.(*embedded.Container)

	// test embedded properties
	require.Equal(t, "mine", cont.Meta.Name)
	require.Equal(t, "mycontainer", cont.ID)
	require.Contains(t, cont.Entrypoint, "echo")
	require.Contains(t, cont.Command, "hello")
	require.Equal(t, "value", cont.Env["NAME"])
	require.Contains(t, cont.DNS, "container-dns")
	require.True(t, cont.Privileged)
	require.Equal(t, 5, cont.MaxRestartCount)

	// test specific properties
	require.Equal(t, "mycontainer", cont.ContainerID)

	r2, err := c.FindResource("resource.sidecar.mine")
	require.NoError(t, err)

	side := r2.(*embedded.Sidecar)

	// test embedded properties
	require.Equal(t, "mine", side.Meta.Name)
	require.Equal(t, "mycontainer", side.ID)
	require.Contains(t, side.Entrypoint, "echo")
	require.Contains(t, side.Command, "hello")
	require.Equal(t, "value", side.Env["NAME"])
	require.Contains(t, side.DNS, "container-dns")
	require.False(t, side.Privileged)
	require.Equal(t, 3, side.MaxRestartCount)

	// test specific properties
	require.Equal(t, "mysidecar", side.SidecarID)
}

func TestParseParsesToResourceBase(t *testing.T) {
	// Test that when PrimativesOnly is set the configuration is parsed
	// into ResouceBase not registered types

	f, pathErr := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if pathErr != nil {
		t.Fatal(pathErr)
	}

	o := DefaultOptions()
	o.PrimativesOnly = true

	p := NewParser(o)

	c, err := p.ParseFile(f)
	require.NoError(t, err)

	require.NotNil(t, c)

	// check we have a Resousece base for the container
	r, err := c.FindResource("module.consul_1.resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, "consul", r.Metadata().Name)
	require.Equal(t, "container", r.Metadata().Type)
	require.Equal(t, "resource.network.onprem.meta.name", r.Metadata().Links[0])

	r, err = c.FindResource("module.consul_2.resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, "consul", r.Metadata().Name)
	require.Equal(t, "container", r.Metadata().Type)
	require.Equal(t, "resource.network.onprem.meta.name", r.Metadata().Links[0])

	r, err = c.FindResource("module.consul_2")
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, "consul_2", r.Metadata().Name)
	require.Equal(t, "module", r.Metadata().Type)

	m1 := r.(*resources.Module)
	require.Equal(t, "../single", m1.Source)
	require.Equal(t, "latest", m1.Version)

	r, err = c.FindResource("module.consul_2.output.container_name")
	require.NoError(t, err)
	require.NotNil(t, r)

	o1 := r.(*resources.Output)
	require.Equal(t, "This is the name of the container", o1.Description)
}
