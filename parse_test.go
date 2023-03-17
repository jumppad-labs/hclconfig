package hclconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/shipyard-run/hclconfig/test_fixtures/structs"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func setupParser(t *testing.T, options ...*ParserOptions) *Parser {
	os.Setenv("SHIPYARD_CONFIG", "/User/yamcha/.shipyard")

	t.Cleanup(func() {
		os.Unsetenv("SHIPYARD_CONFIG")
	})

	o := DefaultOptions()
	if len(options) > 0 {
		o = options[0]
	}

	p := NewParser(o)
	p.RegisterType("container", &structs.Container{})
	p.RegisterType("network", &structs.Network{})
	p.RegisterType("template", &structs.Template{})

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

	cont := r.(*structs.Container)

	require.Equal(t, "consul", cont.Metadata().Name)
	require.Equal(t, absoluteFolderPath, cont.Metadata().File)

	require.Equal(t, "consul", cont.Command[0], "consul")
	require.Equal(t, "10.6.0.200", cont.Networks[0].IPAddress)
	require.Equal(t, 2048, cont.Resources.CPU)

	r, err = c.FindResource("resource.container.base")
	require.NoError(t, err)
	require.NotNil(t, r)
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
	require.Len(t, cont.ResourceLinks, 6)

	require.Contains(t, cont.ResourceLinks, "resource.network.onprem.name")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.dns")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.resources.cpu_pin")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.resources.memory")
	require.Contains(t, cont.ResourceLinks, "resource.template.consul_config.destination")
	require.Contains(t, cont.ResourceLinks, "resource.template.consul_config.name")
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

func TestParseModuleCreatesResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// count the resources, should create 4
	require.Len(t, c.Resources, 13)

	// check resource has been created
	cont, err := c.FindResource("module.consul_1.resource.container.consul")
	require.NoError(t, err)

	// check interpolation value
	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)

	// check resource has been created
	cont, err = c.FindResource("module.consul_2.resource.container.consul")
	require.NoError(t, err)

	// check interpolation value
	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)

	// check outputs
	cont, err = c.FindResource("resource.output.module1_container_resources_cpu")
	require.NoError(t, err)

	// check interpolation value is overriden in the module stanza
	require.Equal(t, "4096", cont.(*types.Output).Value)

	cont, err = c.FindResource("resource.output.module2_container_resources_cpu")
	require.NoError(t, err)

	// check interpolation value
	require.Equal(t, "2048", cont.(*types.Output).Value)
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

func TestParseContainerWithNoLabelReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/invalid/no_name.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)
}

func TestParseDoesNotProcessDisabledResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/disabled/disabled.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.disabled")
	require.NoError(t, err)
	require.True(t, r.Metadata().Disabled)
}

func TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/disabled/module.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("module.disabled.resource.container.enabled")
	require.NoError(t, err)
	require.True(t, r.Metadata().Disabled)

	r, err = c.FindResource("module.disabled.sub.resource.container.enabled")
	require.NoError(t, err)
	require.True(t, r.Metadata().Disabled)
}

func TestParseProcessesDefaultFunctions(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/functions/default.hcl")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MYENV", "myvalue")
	t.Cleanup(func() {
		os.Unsetenv("MYENV")
	})

	home, _ := os.UserHomeDir()

	p := setupParser(t)
	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.base")
	require.NoError(t, err)

	cont := r.(*structs.Container)

	require.Equal(t, "3", cont.Env["len_string"])
	require.Equal(t, "2", cont.Env["len_collection"])
	require.Equal(t, "myvalue", cont.Env["env"])
	require.Equal(t, home, cont.Env["home"])
	require.Contains(t, cont.Env["file"], "container")
	require.Contains(t, cont.Env["dir"], filepath.Dir(absoluteFolderPath))
	require.Contains(t, cont.Env["trim"], "foo bar")
}

func TestParseProcessesCustomFunctions(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/functions/custom.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)
	p.RegisterFunction("constant_number", func() (int, error) { return 42, nil })

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.base")
	require.NoError(t, err)

	cont := r.(*structs.Container)

	require.Equal(t, "42", cont.Env["len"])
}

func TestSetContextVariableFromPath(t *testing.T) {
	ctx := &hcl.EvalContext{}
	ctx.Variables = map[string]cty.Value{"resource": cty.ObjectVal(map[string]cty.Value{})}

	setContextVariableFromPath(ctx, "resource.foo.bar", cty.BoolVal(true))
	setContextVariableFromPath(ctx, "resource.foo.bear", cty.StringVal("Hello World"))
	setContextVariableFromPath(ctx, "resource.poo", cty.StringVal("Meh"))

	require.True(t, ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["bar"].True())
	require.Equal(t, "Hello World", ctx.Variables["resource"].AsValueMap()["foo"].AsValueMap()["bear"].AsString())
	require.Equal(t, "Meh", ctx.Variables["resource"].AsValueMap()["poo"].AsString())
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

		calls = append(calls, ResourceFQDN{
			Module:   r.Metadata().Module,
			Resource: r.Metadata().Name,
			Type:     r.Metadata().Type,
		}.String())

		callSync.Unlock()

		return nil
	}

	p := setupParser(t, o)

	_, err = p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	// check the order, should be ...
	// resource.container.base
	// -- resource.module.consul_1
	// -- -- module.consul_1.resource.network.onprem
	// -- -- -- module.consul_1.resource.container.consul
	// -- -- -- -- module.consul_1.resource.output.container_name
	// -- -- -- -- module.consul_1.resource.output.container_resources_cpu
	// -- -- -- -- -- resource.output.module_1_container_resources_cpu
	// resource.module.consul_2
	// -- module.consul_2.resource.network.onprem
	// -- -- module.consul_2.resource.container.consul
	// -- -- -- module.consul_2.resource.output.container_name
	// -- -- -- module.consul_2.resource.output.container_resources_cpu
	// -- -- -- -- resource.output.module_2_container_resources_cpu

	requireBefore(t, "resource.container.base", "resource.module.consul_1", calls)
	requireBefore(t, "resource.module.consul_2", "module.consul_2.resource.network.onprem", calls)
	requireBefore(t, "module.consul_2.resource.network.onprem", "module.consul_2.resource.container.consul", calls)
	requireBefore(t, "module.consul_1.resource.container.consul", "output.module1_container_resources_cpu", calls)
	requireBefore(t, "module.consul_2.resource.container.consul", "output.module2_container_resources_cpu", calls)
}

func TestParserDesrializesJsonCorrectly(t *testing.T) {
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
