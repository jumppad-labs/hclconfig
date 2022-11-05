package hclconfig

import (
	"path/filepath"
	"testing"

	"github.com/shipyard-run/hclconfig/test_fixtures/single_file/structs"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func setupGraphConfig(t *testing.T) *Config {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/single_file/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	c := NewConfig()

	p := NewParser(DefaultOptions())
	p.RegisterType("container", &structs.Container{})
	p.RegisterType("network", &structs.Network{})
	p.RegisterType("template", &structs.Template{})

	c, err = p.ParseFile(absoluteFolderPath, c)
	require.NoError(t, err)

	return c
}

//	func TestDoYaLikeDAGGeneratesAGraph(t *testing.T) {
//		c := testSetupConfig(t)
//
//		d, err := c.DoYaLikeDAGs()
//		assert.NoError(t, err)
//
//		// check that all resources are added and dependencies created
//		assert.Len(t, d.Edges(), 4)
//	}
func TestDoYaLikeDAGAddsDependencies(t *testing.T) {
	c := setupGraphConfig(t)

	g, err := DoYaLikeDAGs(c)
	require.NoError(t, err)

	// check the dependency tree of the container
	base, _ := c.FindResource("container.base")
	s, err := g.Descendents(base)
	require.NoError(t, err)

	network, _ := c.FindResource("network.onprem")

	// check the network is returned
	list := s.List()
	require.Contains(t, list, network)
}

//
//func TestDoYaLikeDAGAddsDependenciesForModules(t *testing.T) {
//	c := testSetupModuleConfig(t)
//
//	g, err := c.DoYaLikeDAGs()
//	assert.NoError(t, err)
//
//	// check the dependency tree of a cluster
//	s, err := g.Descendents(c.Resources[1])
//	assert.NoError(t, err)
//
//	// check that the network and a blueprint is returned
//	list := s.List()
//	assert.Contains(t, list, c.Resources[0])
//	assert.Contains(t, list, &Blueprint{})
//}
//
//func TestDoYaLikeDAGWithUnresolvedDependencyReturnsError(t *testing.T) {
//	c := testSetupConfig(t)
//
//	con := NewContainer("test")
//	con.DependsOn = []string{"doesnot.exist"}
//
//	c.AddResource(con)
//
//	_, err := c.DoYaLikeDAGs()
//	assert.Error(t, err)
//}

func TestWalkFuncCalledForEveryResource(t *testing.T) {
	c := setupGraphConfig(t)
	callCount := 0

	err := c.Walk(func(r types.Resource) error {
		callCount += 1
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 5, callCount)
}
