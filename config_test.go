package hclconfig

import (
	"testing"

	"github.com/shipyard-run/hclconfig/test_fixtures/structs"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func testSetupConfig(t *testing.T) *Config {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}
	typs[structs.TypeContainer] = &structs.Container{}
	typs[structs.TypeTemplate] = &structs.Template{}

	net1, _ := typs.CreateResource(structs.TypeNetwork, "cloud")
	con1, _ := typs.CreateResource(structs.TypeContainer, "test_dev")

	// depending on a module should return all resouces and
	// all child resources
	con1.Metadata().DependsOn = []string{"module.module1"}

	// onc 2 is embedded in module1
	con2, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	con2.Metadata().Module = "module1"

	// con 3 is loaded from a module inside module1
	con3, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	con3.Metadata().Module = "module1.module2"

	// con 3 is loaded from a module inside module1
	con4, _ := typs.CreateResource(structs.TypeContainer, "test_dev2")
	con4.Metadata().Module = "module1.module2"

	// depends on would be added relative as a resource
	// when a resource is defined, it has no idea on its
	// module
	con4.Metadata().DependsOn = []string{"resource.container.test_dev"}

	out1, _ := typs.CreateResource(types.TypeOutput, "fqdn")
	out1.Metadata().Module = "module1.module2"

	c := newConfig()
	err := c.addResource(net1, nil, nil)
	require.NoError(t, err)

	// ensure the config reference is set
	require.Equal(t, c, net1.Metadata().ParentConfig)

	err = c.addResource(con1, nil, nil)
	require.NoError(t, err)

	err = c.addResource(con2, nil, nil)
	require.NoError(t, err)

	err = c.addResource(con3, nil, nil)
	require.NoError(t, err)

	err = c.addResource(con4, nil, nil)
	require.NoError(t, err)

	err = c.addResource(out1, nil, nil)
	require.NoError(t, err)

	return c
}

func TestResourceCount(t *testing.T) {
	c := testSetupConfig(t)
	require.Equal(t, 6, c.ResourceCount())
}

func TestAddResourceExistsReturnsError(t *testing.T) {
	c := testSetupConfig(t)

	err := c.addResource(c.Resources[3], nil, nil)
	require.Error(t, err)
}

func TestFindResourceFindsContainer(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindResource("resource.container.test_dev")
	require.NoError(t, err)
	require.Equal(t, c.Resources[1], cl)
}

func TestFindResourceFindsModuleOutput(t *testing.T) {
	c := testSetupConfig(t)

	out, err := c.FindResource("module.module1.module2.output.fqdn")
	require.NoError(t, err)
	require.Equal(t, c.Resources[5], out)
}

func TestFindResourceFindsClusterInModule(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindResource("module.module1.resource.container.test_dev")
	require.NoError(t, err)
	require.Equal(t, c.Resources[2], cl)
}

func TestFindRelativeResourceWithParentFindsClusterInModule(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindRelativeResource("resource.container.test_dev", "module1")
	require.NoError(t, err)
	require.Equal(t, c.Resources[2], cl)
}

func TestFindRelativeResourceWithModuleAndParentFindsClusterInModule(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindRelativeResource("module.module2.resource.container.test_dev", "module1")
	require.NoError(t, err)
	require.Equal(t, c.Resources[3], cl)
}

func TestFindRelativeResourceWithModuleAndNoParentFindsClusterInModule(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindRelativeResource("module.module1.resource.container.test_dev", "")
	require.NoError(t, err)
	require.Equal(t, c.Resources[2], cl)
}

func TestFindResourceReturnsNotFoundError(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindResource("resource.container.notexist")
	require.Error(t, err)
	require.IsType(t, ResourceNotFoundError{}, err)
	require.Nil(t, cl)
}

func TestFindResourcesByTypeContainers(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindResourcesByType("container")
	require.NoError(t, err)
	require.Len(t, cl, 4)
}

func TestFindModuleResourcesFindsResources(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindModuleResources("module.module1", false)
	require.NoError(t, err)
	require.Len(t, cl, 1)
}

func TestFindModuleResourcesFindsResourcesWithChildren(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindModuleResources("module.module1", true)
	require.NoError(t, err)
	require.Len(t, cl, 4)
}

func TestFindRelativeModuleResourcesFindsResources(t *testing.T) {
	c := testSetupConfig(t)

	cl, err := c.FindRelativeModuleResources("module.module2", "module1", false)
	require.NoError(t, err)
	require.Len(t, cl, 3)
}

//
//func TestFindDependentResourceFindsResource(t *testing.T) {
//	c := testSetupConfig(t)
//
//	r, err := c.FindResource("k8s_cluster.test.dev")
//	assert.NoError(t, err)
//	assert.Equal(t, c.Resources[1], r)
//}
//

func TestRemoveResourceRemoves(t *testing.T) {
	c := testSetupConfig(t)

	err := c.removeResource(c.Resources[0])
	require.NoError(t, err)
	require.Len(t, c.Resources, 5)
}

func TestRemoveResourceNotFoundReturnsError(t *testing.T) {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}

	c := testSetupConfig(t)
	net1, _ := typs.CreateResource(structs.TypeNetwork, "notfound")

	err := c.removeResource(net1)
	require.Error(t, err)
	require.Len(t, c.Resources, 6)
}

func TestToJSONSerializesJSON(t *testing.T) {
	c := testSetupConfig(t)

	d, err := c.ToJSON()
	require.NoError(t, err)
	require.Greater(t, len(d), 0)

	require.Contains(t, string(d), `"name": "test_dev"`)
}

func TestAppendResourcesMerges(t *testing.T) {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}

	c := testSetupConfig(t)

	c2 := newConfig()
	net1, err := typs.CreateResource(structs.TypeNetwork, "cloud2")
	require.NoError(t, err)
	c2.addResource(net1, nil, nil)

	err = c.AppendResourcesFromConfig(c2)
	require.NoError(t, err)

	net2, err := c.FindResource("resource.network.cloud2")
	require.NoError(t, err)
	require.Equal(t, net1, net2)
}

func TestAppendResourcesWhenExistsReturnsError(t *testing.T) {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}

	c := testSetupConfig(t)

	c2 := newConfig()
	net1, err := typs.CreateResource(structs.TypeNetwork, "cloud")
	require.NoError(t, err)
	c2.addResource(net1, nil, nil)

	err = c.AppendResourcesFromConfig(c2)
	require.Error(t, err)
}
