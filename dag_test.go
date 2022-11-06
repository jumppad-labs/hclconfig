package hclconfig

import (
	"path/filepath"
	"testing"

	"github.com/shipyard-run/hclconfig/test_fixtures/structs"
	"github.com/shipyard-run/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func setupGraphConfig(t *testing.T) *Config {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
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

func TestDoYaLikeDAGAddsDependencies(t *testing.T) {
	c := setupGraphConfig(t)

	g, err := doYaLikeDAGs(c)
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

//func TestDoYaLikeDAGAddsDependenciesForModules(t *testing.T) {
//	c := setupGraphConfig(t)
//
//	g, err := doYaLikeDAGs(c)
//	require.NoError(t, err)
//
//	// check the dependency tree of a cluster
//	s, err := g.Descendents(c.Resources[1])
//	require.NoError(t, err)
//
//	// check that the network and a blueprint is returned
//	list := s.List()
//	require.Contains(t, list, c.Resources[0])
//	require.Contains(t, list, &struct.Container{})
//}

func TestWalkWithUnresolvedDependencyReturnsError(t *testing.T) {
	c := setupGraphConfig(t)

	con := (&structs.Container{}).New("test")
	con.Info().DependsOn = []string{"doesnot.exist"}

	c.AddResource(con)

	err := c.Walk(func(r types.Resource) error {
		return nil
	})

	require.Error(t, err)
}

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

func TestWalkResolvesReferences(t *testing.T) {
	c := setupGraphConfig(t)
	err := c.Walk(func(r types.Resource) error {
		return nil
	})

	require.NoError(t, err)

	cont, err := c.FindResource("container.base")
	require.NoError(t, err)
	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)

	cont, err = c.FindResource("container.consul")
	require.NoError(t, err)
	require.Equal(t, "onprem", cont.(*structs.Container).Networks[0].Name)
	require.Equal(t, 2048, cont.(*structs.Container).Resources.CPU)
	require.Equal(t, 1024, cont.(*structs.Container).Resources.Memory)
	require.Equal(t, "./consul.hcl", cont.(*structs.Container).Volumes[1].Source)
}
