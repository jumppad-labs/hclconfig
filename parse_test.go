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
	require.Equal(t, "something", cont.Properties["status"])
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
	require.Len(t, cont.ResourceLinks, 9)

	require.Contains(t, cont.ResourceLinks, "resource.network.onprem.name")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.dns")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.resources.cpu_pin")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.resources.memory")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.resources.user")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.network[0].id")
	require.Contains(t, cont.ResourceLinks, "resource.container.base.network[1].name")
	require.Contains(t, cont.ResourceLinks, "resource.template.consul_config.destination")
	require.Contains(t, cont.ResourceLinks, "resource.template.consul_config.name")
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
	r, err := c.FindResource("resource.output.ip_address_1")
	require.NoError(t, err)
	require.NotNil(t, r)

	out := r.(*types.Output)
	require.Equal(t, "10.6.0.200", out.Value)

	// check variable has been interpolated
	r, err = c.FindResource("resource.output.ip_address_2")
	require.NoError(t, err)
	require.NotNil(t, r)

	out = r.(*types.Output)
	require.Equal(t, "10.7.0.201", out.Value)
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
	require.Len(t, c.Resources, 19)

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

	// check outputs
	cont, err = c.FindResource("resource.output.module1_container_resources_cpu")
	require.NoError(t, err)

	// check interpolation value is overriden in the module stanza
	require.Equal(t, "4096", cont.(*types.Output).Value)

	cont, err = c.FindResource("resource.output.module2_container_resources_cpu")
	require.NoError(t, err)

	// check interpolation value
	require.Equal(t, "512", cont.(*types.Output).Value)

	cont, err = c.FindResource("resource.output.module3_container_resources_cpu")
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

func TestParseContainerWithNoNameReturnsError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/invalid/no_name.hcl")
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

func TestParseDoesNotProcessDisabledResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/disabled/disabled.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)
	require.Equal(t, 1, c.ResourceCount())

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
	o.ParseCallback = func(r types.Resource) error {
		callSync.Lock()

		calls = append(calls, types.ResourceFQDN{
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
	// -- -- -- -- -- -- resource.module.consul_3
	// -- -- -- -- -- -- -- module.consul_3.resource.network.onprem
	// -- -- -- -- -- -- -- -- module.consul_3.resource.container.consul
	// -- -- -- -- -- -- -- -- -- module.consul_3.resource.output.container_name
	// -- -- -- -- -- -- -- -- -- module.consul_3.resource.output.container_resources_cpu
	// -- -- -- -- -- -- -- -- -- -- resource.output.module_1_container_resources_cpu
	// resource.module.consul_2
	// -- module.consul_2.resource.network.onprem
	// -- -- module.consul_2.resource.container.consul
	// -- -- -- module.consul_2.resource.output.container_name
	// -- -- -- module.consul_2.resource.output.container_resources_cpu
	// -- -- -- -- resource.output.module_2_container_resources_cpu

	// module1 depends on an attribute of resource.container.base, all resources in module1 should only
	// be processed after container.base has been created
	requireBefore(t, "resource.container.base", "resource.module.consul_1", calls)

	// resource.network.onprem in module.consul_2 should be created after the top level module is created
	requireBefore(t, "resource.module.consul_2", "module.consul_2.resource.network.onprem", calls)

	// resource.container.consul in module consul_2 depends on resource.network.onprem in module2 it should always
	// be created after the network
	requireBefore(t, "module.consul_2.resource.network.onprem", "module.consul_2.resource.container.consul", calls)

	// the output module_1_container_resources_cpu depends on an output defined in module consul_1, it should always be created
	// after all resources in module consul_1
	requireBefore(t, "module.consul_1.resource.container.consul", "output.module1_container_resources_cpu", calls)

	// the output module_2_container_resources_cpu depends on an output defined in module consul_2, it should always be created
	// after all resources in module consul_2
	requireBefore(t, "module.consul_2.resource.container.consul", "output.module2_container_resources_cpu", calls)

	// the module consul_3 has a hard coded dependency on module_1, it should only be created after all
	// resources in module_1 have been created
	requireBefore(t, "module.consul_1.output.container_resources_cpu", "resource.module.consul_3", calls)
}

func TestParserStopsParseOnCallbackError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/modules.hcl")
	if err != nil {
		t.Fatal(err)
	}

	o := DefaultOptions()
	calls := []string{}
	callSync := sync.Mutex{}
	o.ParseCallback = func(r types.Resource) error {
		callSync.Lock()

		calls = append(calls, types.ResourceFQDN{
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

	// only 7 of the resources should be created, none of the descendants of base
	require.Len(t, calls, 7)
	require.NotContains(t, "resource.module.consul_1", calls)
}

func TestParserDesrializesJSONCorrectly(t *testing.T) {
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

func TestParserErrorOutputsString(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/simple/container.hcl")
	require.NoError(t, pathErr)

	err := ParserError{}
	err.Line = 80
	err.Column = 18
	err.Filename = f
	err.Message = "something has gone wrong, Erik probably made a typo somewhere, nic will have to fix"

	require.Contains(t, err.Error(), "Error:")
	require.Contains(t, err.Error(), "80")
}
