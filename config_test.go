package hclconfig

//func testSetupConfig(t *testing.T) *Config {
//	cache := (&types.ImageCache{}).New("docker-cache")
//	net1 := (&types.Network{}).New("cloud")
//	cl1 := (&types.K8sCluster{}).New("test.dev")
//	cl1.Info().DependsOn = []string{"network.cloud"}
//	cache.Info().DependsOn = []string{"network.cloud"}
//
//	cl2 := (&types.K8sCluster{}).New("test.dev")
//	cl2.Info().Module = "sub_module"
//
//	c := NewConfig()
//	c.AddResource(net1)
//	c.AddResource(cl1)
//	c.AddResource(cache)
//	c.AddResource(cl2)
//
//	return c
//}
//
//func testSetupModuleConfig(t *testing.T) *Config {
//	net1 := (&types.Network{}).New("cloud")
//	net1.Info().Module = "test"
//
//	cl1 := (&types.K8sCluster{}).New("test.dev")
//	cl1.Info().DependsOn = []string{"module.test"}
//
//	c := NewConfig()
//	err := c.AddResource(net1)
//	assert.NoError(t, err)
//
//	err = c.AddResource(cl1)
//	assert.NoError(t, err)
//
//	return c
//}
//
//func TestResourceCount(t *testing.T) {
//
//	//assert.Equal(t, 10, c.ResourceCount())
//}
//
//func TestResourceAddChildSetsDetails(t *testing.T) {
//	c := testSetupConfig(t)
//	cl := (&types.K8sCluster{}).New("newtest")
//
//	c.AddResource(cl)
//
//	assert.Equal(t, c.Resources[0].Info().Type, cl.Info().Type)
//}
//
//func TestFindResourceFindsCluster(t *testing.T) {
//	c := testSetupConfig(t)
//
//	cl, err := c.FindResource("k8s_cluster.test.dev")
//	assert.NoError(t, err)
//	assert.Equal(t, c.Resources[1], cl)
//}
//
//func TestFindResourceFindsClusterInModule(t *testing.T) {
//	c := testSetupConfig(t)
//
//	cl, err := c.FindResource("sub_module.k8s_cluster.test.dev")
//	assert.NoError(t, err)
//	assert.Equal(t, c.Resources[3], cl)
//}
//
//func TestFindResourceReturnsNotFoundError(t *testing.T) {
//	c := testSetupConfig(t)
//
//	cl, err := c.FindResource("container.notexist")
//	assert.Error(t, err)
//	fmt.Println(err)
//	assert.IsType(t, ResourceNotFoundError{}, err)
//	assert.Nil(t, cl)
//}
//
//func TestFindDependentResourceFindsResource(t *testing.T) {
//	c := testSetupConfig(t)
//
//	r, err := c.FindResource("k8s_cluster.test.dev")
//	assert.NoError(t, err)
//	assert.Equal(t, c.Resources[1], r)
//}
//
//func TestAddResourceAddsAResouce(t *testing.T) {
//	c := testSetupConfig(t)
//
//	cl := (&types.K8sCluster{}).New("mikey")
//	err := c.AddResource(cl)
//	assert.NoError(t, err)
//
//	cl2, err := c.FindResource("k8s_cluster.mikey")
//	assert.NoError(t, err)
//	assert.Equal(t, cl, cl2)
//}
//
//func TestAddResourceExistsReturnsError(t *testing.T) {
//	c := testSetupConfig(t)
//
//	err := c.AddResource(c.Resources[3])
//	assert.Error(t, err)
//}
//
//func TestAddResourceDifferentModuleSameNameOK(t *testing.T) {
//	c := testSetupConfig(t)
//
//	cl1 := (&types.K8sCluster{}).New("test.dev")
//	cl1.Info().Module = "mymodule"
//
//	err := c.AddResource(cl1)
//	assert.NoError(t, err)
//}
//
//func TestRemoveResourceRemoves(t *testing.T) {
//	c := testSetupConfig(t)
//
//	err := c.RemoveResource(c.Resources[0])
//	assert.NoError(t, err)
//	assert.Len(t, c.Resources, 3)
//}
//
//func TestRemoveResourceNotFoundReturnsError(t *testing.T) {
//	c := testSetupConfig(t)
//
//	err := c.RemoveResource(nil)
//	assert.Error(t, err)
//	assert.Len(t, c.Resources, 4)
//}
