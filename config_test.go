package hclconfig

import (
	"fmt"
	"sync"
	"testing"

	"github.com/jumppad-labs/hclconfig/test_fixtures/structs"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func testSetupConfig(t *testing.T) (*Config, []types.Resource) {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}
	typs[structs.TypeContainer] = &structs.Container{}
	typs[structs.TypeTemplate] = &structs.Template{}

	var1, _ := typs.CreateResource(types.TypeVariable, "var1")
	var1.Metadata().Checksum = types.Checksum{
		Parsed:    "123",
		Processed: "abc",
	}

	net1, _ := typs.CreateResource(structs.TypeNetwork, "cloud")
	net1.Metadata().Checksum = types.Checksum{
		Parsed:    "234",
		Processed: "bcd",
	}

	mod1, _ := typs.CreateResource(types.TypeModule, "module1")
	mod1.Metadata().DependsOn = []string{"resource.network.cloud"}
	mod1.Metadata().Checksum = types.Checksum{
		Parsed:    "345",
		Processed: "cde",
	}

	var2, _ := typs.CreateResource(types.TypeVariable, "var2")
	var2.Metadata().Module = "module1"
	var2.Metadata().Checksum = types.Checksum{
		Parsed:    "456",
		Processed: "def",
	}

	mod2, _ := typs.CreateResource(types.TypeModule, "module2")
	mod2.Metadata().Module = "module1"
	mod2.Metadata().Checksum = types.Checksum{
		Parsed:    "567",
		Processed: "efg",
	}

	// depending on a module should return all resources and
	// all child resources
	con1, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	con1.Metadata().DependsOn = []string{"module.module1"}
	con1.Metadata().Checksum = types.Checksum{
		Parsed:    "678",
		Processed: "fgh",
	}

	// con2 is embedded in module1
	con2, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	con2.Metadata().Module = "module1"
	con2.Metadata().Checksum = types.Checksum{
		Parsed:    "789",
		Processed: "ghi",
	}

	// con3 is loaded from a module inside module2
	con3, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	con3.Metadata().Module = "module1.module2"
	con3.Metadata().Checksum = types.Checksum{
		Parsed:    "890",
		Processed: "hij",
	}

	// con4 is loaded from a module inside module2
	con4, _ := typs.CreateResource(structs.TypeContainer, "test_dev2")
	con4.Metadata().Module = "module1.module2"

	// depends on would be added relative as a resource
	// when a resource is defined, it has no idea on its
	// module
	con4.Metadata().DependsOn = []string{"resource.container.test_dev"}
	con4.Metadata().Checksum = types.Checksum{
		Parsed:    "90a",
		Processed: "ijk",
	}

	out1, _ := typs.CreateResource(types.TypeOutput, "fqdn")
	out1.Metadata().Module = "module1.module2"
	out1.Metadata().Checksum = types.Checksum{
		Parsed:    "0ab",
		Processed: "jkl",
	}

	out2, _ := typs.CreateResource(types.TypeOutput, "out")
	out2.Metadata().DependsOn = []string{"resource.network.cloud.id", "resource.container.test_dev"}
	out2.Metadata().Checksum = types.Checksum{
		Parsed:    "abc",
		Processed: "klm",
	}

	c := NewConfig()
	err := c.addResource(net1, nil, nil)
	require.NoError(t, err)

	err = c.addResource(var1, nil, nil)
	require.NoError(t, err)

	// add the modules
	err = c.addResource(mod1, nil, nil)
	require.NoError(t, err)

	err = c.addResource(var2, nil, nil)
	require.NoError(t, err)

	err = c.addResource(mod2, nil, nil)
	require.NoError(t, err)

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

	err = c.addResource(out2, nil, nil)
	require.NoError(t, err)

	return c, []types.Resource{
		net1,
		con1,
		mod1,
		mod2,
		con2,
		con3,
		con4,
		out1,
		out2,
		var1,
		var2,
	}
}

func TestResourceCount(t *testing.T) {
	c, r := testSetupConfig(t)
	require.Equal(t, len(r), c.ResourceCount())
}

func TestAddResourceExistsReturnsError(t *testing.T) {
	c, r := testSetupConfig(t)

	err := c.AppendResource(r[3])
	require.Error(t, err)
}

func TestFindResourceFindsContainer(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("resource.container.test_dev")
	require.NoError(t, err)
	require.Equal(t, r[1], cl)
}

func TestFindResourceFindsVariable(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("variable.var1")
	require.NoError(t, err)
	require.Equal(t, r[9], cl)
}

func TestFindResourceFindsModuleVariable(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("module.module1.variable.var2")
	require.NoError(t, err)
	require.Equal(t, r[10], cl)
}

func TestFindOutputFindsOutput(t *testing.T) {
	c, _ := testSetupConfig(t)

	_, err := c.FindResource("output.out")
	require.NoError(t, err)
}

func TestFindOutputFindsModule(t *testing.T) {
	c, _ := testSetupConfig(t)

	_, err := c.FindResource("module.module1")
	require.NoError(t, err)
}

func TestFindResourceFindsModuleOutput(t *testing.T) {
	c, r := testSetupConfig(t)

	out, err := c.FindResource("module.module1.module2.output.fqdn")
	require.NoError(t, err)
	require.Equal(t, r[7], out)
}

func TestFindResourceFindsModuleOutputWithIndex(t *testing.T) {
	c, r := testSetupConfig(t)

	out, err := c.FindResource("module.module1.module2.output.fqdn.0")
	require.NoError(t, err)
	require.Equal(t, r[7], out)
}

func TestFindResourceFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("module.module1.resource.container.test_dev")
	require.NoError(t, err)
	require.Equal(t, r[4], cl)
}

func TestFindRelativeResourceWithParentFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindRelativeResource("resource.container.test_dev", "module1")
	require.NoError(t, err)
	require.Equal(t, r[4], cl)
}

func TestFindRelativeResourceWithModuleAndParentFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindRelativeResource("module.module2.resource.container.test_dev", "module1")
	require.NoError(t, err)
	require.Equal(t, r[5], cl)
}

func TestFindRelativeResourceWithModuleAndNoParentFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindRelativeResource("module.module1.resource.container.test_dev", "")
	require.NoError(t, err)
	require.Equal(t, r[4], cl)
}

func TestFindResourceReturnsNotFoundError(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindResource("resource.container.notexist")
	require.Error(t, err)
	require.IsType(t, ResourceNotFoundError{}, err)
	require.Nil(t, cl)
}

func TestFindResourcesByTypeContainers(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindResourcesByType("container")
	require.NoError(t, err)
	require.Len(t, cl, 4)
}

func TestFindModuleResourcesFindsResources(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindModuleResources("module.module1", false)
	require.NoError(t, err)

	// should have one resource and one module
	require.Len(t, cl, 3)
}

func TestFindModuleResourcesFindsResourcesWithChildren(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindModuleResources("module.module1", true)
	require.NoError(t, err)
	require.Len(t, cl, 6)
}

func TestRemoveResourceRemoves(t *testing.T) {
	c, _ := testSetupConfig(t)

	err := c.RemoveResource(c.Resources[0])
	require.NoError(t, err)
	require.Len(t, c.Resources, 10)
}

func TestRemoveResourceNotFoundReturnsError(t *testing.T) {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}

	c, _ := testSetupConfig(t)
	net1, _ := typs.CreateResource(structs.TypeNetwork, "notfound")

	err := c.RemoveResource(net1)
	require.Error(t, err)
	require.Len(t, c.Resources, 11)
}

func TestToJSONSerializesJSON(t *testing.T) {
	c, _ := testSetupConfig(t)

	d, err := c.ToJSON()
	require.NoError(t, err)
	require.Greater(t, len(d), 0)

	require.Contains(t, string(d), `"name": "test_dev"`)
}

func TestAppendResourcesMerges(t *testing.T) {
	typs := types.DefaultTypes()
	typs[structs.TypeNetwork] = &structs.Network{}

	c, _ := testSetupConfig(t)

	c2 := NewConfig()
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

	c, _ := testSetupConfig(t)

	c2 := NewConfig()
	net1, err := typs.CreateResource(structs.TypeNetwork, "cloud")
	require.NoError(t, err)
	c2.addResource(net1, nil, nil)

	err = c.AppendResourcesFromConfig(c2)
	require.Error(t, err)
}

func TestProcessForwardExecutesCallbacksInCorrectOrder(t *testing.T) {
	c, _ := testSetupConfig(t)

	calls := []string{}
	callSync := sync.Mutex{}
	err := c.Walk(
		func(r types.Resource) error {
			callSync.Lock()

			calls = append(calls, types.ResourceFQRN{
				Module:   r.Metadata().Module,
				Resource: r.Metadata().Name,
				Type:     r.Metadata().Type,
			}.String())

			callSync.Unlock()

			return nil
		},
		false,
	)

	require.NoError(t, err)

	// test_dev depends on cloud so should always be called after it
	requireBefore(t, "resource.network.cloud", "module.module1.resource.container.test_dev", calls)

	// out.out depends on resource.container.test_dev depends on module 1 so the container should be called last
	// after all resources in module 1 have been created
	require.Equal(t, "output.out", calls[6])
}

func TestProcessReverseExecutesCallbacksInCorrectOrder(t *testing.T) {
	c, _ := testSetupConfig(t)

	calls := []string{}
	callSync := sync.Mutex{}
	err := c.Walk(
		func(r types.Resource) error {
			callSync.Lock()

			calls = append(calls, types.ResourceFQRN{
				Module:   r.Metadata().Module,
				Resource: r.Metadata().Name,
				Type:     r.Metadata().Type,
			}.String())

			callSync.Unlock()

			return nil
		},
		true,
	)

	require.NoError(t, err)

	// resource.container.test_dev depends on module.module1 so the call back for test_dev
	// should happen first before anything else
	require.Equal(t, "output.out", calls[0])
	requireBefore(t, "resource.container.test_dev", "module.module1.module2.output.fqdn", calls)
}

func TestProcessCallbackErrorHaltsExecution(t *testing.T) {
	c, _ := testSetupConfig(t)

	calls := []string{}
	callSync := sync.Mutex{}
	err := c.Walk(
		func(r types.Resource) error {
			callSync.Lock()
			calls = append(calls, types.ResourceFQRN{
				Module:   r.Metadata().Module,
				Resource: r.Metadata().Name,
				Type:     r.Metadata().Type,
			}.String())

			callSync.Unlock()

			if r.Metadata().Name == "cloud" {
				return fmt.Errorf("boom")
			}

			return nil
		},
		false,
	)

	// we should get an error from process
	require.Error(t, err)

	// process should stop the callbacks, there should only
	// be one callback network cloud
	require.Equal(t, 1, len(calls))
}

func copyConfig(t *testing.T, c *Config) *Config {
	d, _ := c.ToJSON()
	p := setupParser(t)

	new, err := p.UnmarshalJSON(d)
	require.NoError(t, err)
	require.NotNil(t, new)

	return new
}

func TestDiffReturnsResourcesAdded(t *testing.T) {
	c, _ := testSetupConfig(t)
	new := copyConfig(t, c)

	typs := types.DefaultTypes()
	var1, _ := typs.CreateResource(types.TypeVariable, "var22")
	var1.Metadata().Checksum = types.Checksum{
		Parsed:    "zzz",
		Processed: "111",
	}
	new.AppendResource(var1)

	changes, err := c.Diff(new)
	require.NoError(t, err)
	require.NotNil(t, changes)
	require.Len(t, changes.Added, 1)
	require.Len(t, changes.ParseUpdated, 0)
	require.Len(t, changes.ProcessedUpdated, 0)
	require.Len(t, changes.Removed, 0)
	require.Len(t, changes.Unchanged, 11)
}

func TestDiffReturnsResourcesUpdated(t *testing.T) {
	c, _ := testSetupConfig(t)
	new := copyConfig(t, c)

	var1, _ := new.FindResource("variable.var1")
	var1.Metadata().Checksum.Parsed = "zzz"

	var2, _ := new.FindResource("module.module1.variable.var2")
	var2.Metadata().Checksum.Processed = "zzz"

	changes, err := c.Diff(new)
	require.NoError(t, err)
	require.NotNil(t, changes)
	require.Len(t, changes.Added, 0)
	require.Len(t, changes.ParseUpdated, 1)
	require.Len(t, changes.ProcessedUpdated, 1)
	require.Len(t, changes.Removed, 0)
	require.Len(t, changes.Unchanged, 9)
}

func TestDiffReturnsResourcesRemoved(t *testing.T) {
	c, _ := testSetupConfig(t)
	new := copyConfig(t, c)

	var1, _ := new.FindResource("variable.var1")
	var1.Metadata().Checksum = types.Checksum{
		Parsed:    "zzz",
		Processed: "111",
	}

	new.RemoveResource(var1)

	changes, err := c.Diff(new)
	require.NoError(t, err)
	require.NotNil(t, changes)
	require.Len(t, changes.Added, 0)
	require.Len(t, changes.ParseUpdated, 0)
	require.Len(t, changes.ProcessedUpdated, 0)
	require.Len(t, changes.Removed, 1)
	require.Len(t, changes.Unchanged, 10)
}

func TestDiffReturnsResourcesUnchanged(t *testing.T) {
	c, _ := testSetupConfig(t)
	new := copyConfig(t, c)

	changes, err := c.Diff(new)
	require.NoError(t, err)
	require.NotNil(t, changes)
	require.Len(t, changes.Added, 0)
	require.Len(t, changes.ParseUpdated, 0)
	require.Len(t, changes.ProcessedUpdated, 0)
	require.Len(t, changes.Removed, 0)
	require.Len(t, changes.Unchanged, 11)
}
