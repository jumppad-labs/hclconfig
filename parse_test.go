package hclconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/shipyard-run/hclconfig/test_fixtures/structs"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func setupParser(t *testing.T, options ...*ParserOptions) (*Config, *Parser) {
	os.Setenv("SHIPYARD_CONFIG", "/User/yamcha/.shipyard")

	t.Cleanup(func() {
		os.Unsetenv("SHIPYARD_CONFIG")
	})

	c := NewConfig()

	o := DefaultOptions()
	if len(options) > 0 {
		o = options[0]
	}

	o.Callback = func(r types.Resource) error {
		//fmt.Printf("Process %s.%s.%s\n", r.Info().Module, r.Info().Type, r.Info().Name)
		return nil
	}

	p := NewParser(o)
	p.RegisterType("container", &structs.Container{})
	p.RegisterType("network", &structs.Network{})
	p.RegisterType("template", &structs.Template{})

	return c, p
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

	c, p := setupParser(t)

	c, err = p.ParseFile(absoluteFolderPath, c)
	require.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)

	cont := r.(*structs.Container)

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

	c, p := setupParser(t)

	c, err = p.ParseFile(absoluteFolderPath, c)
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
	require.Len(t, cont.ResouceLinks, 6)

	require.Contains(t, cont.ResouceLinks, "resource.network.onprem.name")
	require.Contains(t, cont.ResouceLinks, "resource.container.base.dns")
	require.Contains(t, cont.ResouceLinks, "resource.container.base.resources.cpu_pin")
	require.Contains(t, cont.ResouceLinks, "resource.container.base.resources.memory")
	require.Contains(t, cont.ResouceLinks, "resource.template.consul_config.destination")
	require.Contains(t, cont.ResouceLinks, "resource.template.consul_config.name")
}

func TestLoadsVariableFilesInOptionsOverridingVariableDefaults(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple")
	require.NoError(t, err)

	o := DefaultOptions()
	o.VariablesFiles = []string{filepath.Join(absoluteFolderPath, "vars", "override.vars")}

	c, p := setupParser(t, o)

	c, err = p.ParseFile(filepath.Join(absoluteFolderPath, "container.hcl"), c)
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

	c, p := setupParser(t)

	os.Setenv("HCL_VAR_cpu_resources", "1000")

	t.Cleanup(func() {
		os.Unsetenv("HCL_VAR_cpu_resources")
	})

	c, err = p.ParseFile(filepath.Join(absoluteFolderPath, "container.hcl"), c)
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

	c, p := setupParser(t)

	c, err = p.ParseDirectory(absoluteFolderPath, c)
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

	c, p := setupParser(t)

	c, err = p.ParseDirectory(absoluteFolderPath, c)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	// check variable has been interpolated using the override value
	cont := r.(*structs.Container)
	require.Equal(t, 1024, cont.Resources.CPU)
}

//func TestVariablesSetFromDefaultModule(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/variables/with_module/")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("consul.container.consul")
//	assert.NoError(t, err)
//
//	con := r.(*Container)
//
//	assert.Equal(t, "modulenetwork", con.Networks[0].Name)
//}
//
//func TestOverridesVariablesSetFromDefaultModuleWithEnv(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/variables/with_module/")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	os.Setenv("SY_VAR_mod_network", "cloud")
//	t.Cleanup(func() {
//		os.Unsetenv("SY_VAR_mod_network")
//	})
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("consul.container.consul")
//	assert.NoError(t, err)
//
//	con := r.(*Container)
//	assert.Equal(t, "cloud", con.Networks[0].Name)
//}
//
//func TestDoesNotLoadsVariablesFilesFromInsideModules(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/modules")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("consul.container.consul")
//	assert.NoError(t, err)
//
//	validEnv := false
//	con := r.(*Container)
//	for _, e := range con.Environment {
//		fmt.Println(e.Value)
//		// should contain a key called "something" with a value "else"
//		if e.Key == "something" && e.Value == "this is a module" {
//			validEnv = true
//		}
//	}
//
//	assert.True(t, validEnv)
//}
//

func TestParseModuleCreatesResources(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/modules/")
	if err != nil {
		t.Fatal(err)
	}

	c, p := setupParser(t)

	c, err = p.ParseDirectory(absoluteFolderPath, c)
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

//
//func TestParseFileFunctionReadCorrectly(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/container")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("container.consul")
//	assert.NoError(t, err)
//
//	validEnv := false
//	con := r.(*Container)
//	for _, e := range con.Environment {
//		// should contain a key called "something" with a value "else"
//		if e.Key == "file" && e.Value == "this is the contents of a file" {
//			validEnv = true
//		}
//	}
//
//	assert.True(t, validEnv)
//}
//
//func TestParseContainerWithNoLabelReturnsError(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/invalid/container.hcl")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//	err = ParseSingleFile(absoluteFolderPath, c, map[string]string{}, "")
//	assert.Error(t, err)
//}
//
//func TestParseAddsCacheDependencyToK8sResources(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/single_k3s_cluster")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("k8s_cluster.k3s")
//	assert.NoError(t, err)
//
//	assert.Contains(t, r.Info().DependsOn, fmt.Sprintf("%s.%s", string(TypeImageCache), utils.CacheResourceName))
//}
//
//func TestParseAddsCacheDependencyToNomadResources(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/nomad")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("nomad_cluster.dev")
//	assert.NoError(t, err)
//
//	assert.Contains(t, r.Info().DependsOn, fmt.Sprintf("%s.%s", string(TypeImageCache), utils.CacheResourceName))
//}
//
//func TestParseProcessesDisabled(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/container")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// count the resources, should create 10
//	assert.Len(t, c.Resources, 7)
//
//	// check depends on is set
//	r, err := c.FindResource("container.consul_disabled")
//	assert.NoError(t, err)
//	assert.Equal(t, r.Info().Disabled, true)
//	assert.Equal(t, Disabled, r.Info().Status)
//}
//
//func TestParseProcessesDisabledOnModuleSettingChildDisabled(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("../../examples/modules")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
//	assert.NoError(t, err)
//
//	// count the resources, should create 21
//	assert.Len(t, c.Resources, 21)
//
//	// check depends on is set
//	r, err := c.FindResource("consul.container.consul_disabled")
//	assert.NoError(t, err)
//	assert.Equal(t, true, r.Info().Disabled)
//
//	//
//	r, err = c.FindResource("k8s_exec.exec_local.run")
//	assert.NoError(t, err)
//	assert.Equal(t, true, r.Info().Disabled)
//}
//
//func TestParseProcessesShipyardFunctions(t *testing.T) {
//	tDir := t.TempDir()
//	home := os.Getenv(utils.HomeEnvName())
//	os.Setenv(utils.HomeEnvName(), tDir)
//	t.Cleanup(func() {
//		os.Setenv(utils.HomeEnvName(), home)
//	})
//
//	absoluteFolderPath, err := filepath.Abs("../../examples/functions")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	absoluteFilePath, err := filepath.Abs("../../examples/functions/container.hcl")
//	assert.NoError(t, err)
//
//	absoluteVarsPath, err := filepath.Abs("../../examples/override.vars")
//	assert.NoError(t, err)
//
//	_, kubeConfigFile, kubeConfigDockerFile := utils.CreateKubeConfigPath("dc1")
//
//	ip, _ := utils.GetLocalIPAndHostname()
//	clusterConf, _ := utils.GetClusterConfig("nomad_cluster.dc1")
//	clusterIP := clusterConf.APIAddress(utils.LocalContext)
//	clusterPort := fmt.Sprintf("%d", clusterConf.RemoteAPIPort)
//
//	c := New()
//	err = ParseSingleFile(absoluteFilePath, c, map[string]string{}, absoluteVarsPath)
//	assert.NoError(t, err)
//
//	// check variable has been interpolated
//	r, err := c.FindResource("container.consul")
//	assert.NoError(t, err)
//
//	cc := r.(*Container)
//
//	assert.Equal(t, absoluteFolderPath, cc.EnvVar["file_dir"])
//	assert.Equal(t, os.Getenv("HOME"), cc.EnvVar["env"])
//	assert.Equal(t, kubeConfigFile, cc.EnvVar["k8s_config"])
//	assert.Equal(t, kubeConfigDockerFile, cc.EnvVar["k8s_config_docker"])
//	assert.Equal(t, os.Getenv("HOME"), cc.EnvVar["home"])
//	assert.Equal(t, utils.ShipyardHome(), cc.EnvVar["shipyard"])
//	assert.Contains(t, cc.EnvVar["file"], "version=\"consul:1.8.1\"")
//	assert.Equal(t, utils.GetDataFolder("mine", os.ModePerm), cc.EnvVar["data"])
//	assert.Equal(t, utils.GetDockerIP(), cc.EnvVar["docker_ip"])
//	assert.Equal(t, utils.GetDockerHost(), cc.EnvVar["docker_host"])
//	assert.Equal(t, ip, cc.EnvVar["shipyard_ip"])
//	assert.Equal(t, clusterIP, cc.EnvVar["cluster_api"])
//	assert.Equal(t, clusterPort, cc.EnvVar["cluster_port"])
//	assert.Equal(t, "2", cc.EnvVar["var_len"])
//}
//
///*
//func TestSingleKubernetesCluster(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("./examples/single-cluster-k8s")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	tearDown := setup()
//	defer tearDown()
//
//	c := New()
//	err = ParseFolder("./examples/single-cluster-k8s", c)
//
//	assert.NoError(t, err)
//	assert.NotNil(t, c)
//
//	// validate clusters
//	assert.Len(t, c.Clusters, 1)
//
//	c1 := c.Clusters[0]
//	assert.Equal(t, "default", c1.Name)
//	assert.Equal(t, "1.16.0", c1.Version)
//	assert.Equal(t, 3, c1.Nodes)
//	assert.Equal(t, "network.k8s", c1.Network)
//
//	// validate networks
//	assert.Len(t, c.Networks, 1)
//
//	n1 := c.Networks[0]
//	assert.Equal(t, "k8s", n1.Name)
//	assert.Equal(t, "10.4.0.0/16", n1.Subnet)
//
//	// validate helm charts
//	assert.Len(t, c.HelmCharts, 1)
//
//	h1 := c.HelmCharts[0]
//	assert.Equal(t, "cluster.default", h1.Cluster)
//	assert.Equal(t, "/User/yamcha/.shipyard/charts/consul", h1.Chart)
//	assert.Equal(t, fmt.Sprintf("%s/consul-values", absoluteFolderPath), h1.Values)
//	assert.Equal(t, "component=server,app=consul", h1.HealthCheck.Pods[0])
//	assert.Equal(t, "component=client,app=consul", h1.HealthCheck.Pods[1])
//
//	// validate ingress
//	assert.Len(t, c.Ingresses, 2)
//
//	i1 := c.Ingresses[0]
//	assert.Equal(t, "consul", i1.Name)
//	assert.Equal(t, 8500, i1.Ports[0].Local)
//	assert.Equal(t, 8500, i1.Ports[0].Remote)
//	assert.Equal(t, 8500, i1.Ports[0].Host)
//
//	i2 := c.Ingresses[1]
//	assert.Equal(t, "web", i2.Name)
//
//	// validate references
//	err = ParseReferences(c)
//	assert.NoError(t, err)
//
//	assert.Equal(t, n1, c1.NetworkRef)
//	assert.Equal(t, c.WAN, c1.WANRef)
//	assert.Equal(t, c1, h1.ClusterRef)
//	assert.Equal(t, i1.TargetRef, c1)
//	assert.Equal(t, i1.NetworkRef, n1)
//	assert.Equal(t, c.WAN, i1.WANRef)
//	assert.Equal(t, i2.TargetRef, c1)
//}
//
//func TestMultiCluster(t *testing.T) {
//	absoluteFolderPath, err := filepath.Abs("./examples/multi-cluster")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	tearDown := setup()
//	defer tearDown()
//
//	c := New()
//	err = ParseFolder(absoluteFolderPath, c)
//
//	assert.NoError(t, err)
//	assert.NotNil(t, c)
//
//	// validate clusters
//	assert.Len(t, c.Clusters, 2)
//
//	c1 := c.Clusters[0]
//	assert.Equal(t, "cloud", c1.Name)
//	assert.Equal(t, "1.16.0", c1.Version)
//	assert.Equal(t, 1, c1.Nodes)
//	assert.Equal(t, "network.k8s", c1.Network)
//
//	// validate containers
//	assert.Len(t, c.Containers, 2)
//
//	co1 := c.Containers[0]
//	assert.Equal(t, "consul_nomad", co1.Name)
//	assert.Equal(t, []string{"consul", "agent", "-config-file=/config/consul.hcl"}, co1.Command)
//	assert.Equal(t, fmt.Sprintf("%s/consul_config", absoluteFolderPath), co1.Volumes[0].Source, "Volume should have been converted to be absolute")
//	assert.Equal(t, "/config", co1.Volumes[0].Destination)
//	assert.Equal(t, "network.nomad", co1.Network)
//	assert.Equal(t, "10.6.0.2", co1.IPAddress)
//
//	// validate ingress
//	assert.Len(t, c.Ingresses, 6)
//
//	i1 := testFindIngress("consul_nomad", c.Ingresses)
//	assert.Equal(t, "consul_nomad", i1.Name)
//
//	// validate references
//	err = ParseReferences(c)
//	assert.NoError(t, err)
//
//	assert.Equal(t, co1, i1.TargetRef)
//	assert.Equal(t, c.WAN, i1.WANRef)
//
//	// validate documentation
//	d1 := c.Docs
//	assert.Equal(t, "multi-cluster", d1.Name)
//	assert.Equal(t, fmt.Sprintf("%s/docs", absoluteFolderPath), d1.Path)
//	assert.Equal(t, 8080, d1.Port)
//	assert.Equal(t, "index.html", d1.Index)
//}
//
//func testFindIngress(name string, ingress []*Ingress) *Ingress {
//	for _, i := range ingress {
//		if i.Name == name {
//			return i
//		}
//	}
//
//	return nil
//}
//*/

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
