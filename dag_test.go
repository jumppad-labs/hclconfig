package hclconfig

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/hclconfig/errors"
	"github.com/jumppad-labs/hclconfig/test_fixtures/structs"
	"github.com/stretchr/testify/require"
)

func setupGraphConfig(t *testing.T) *Config {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := NewParser(DefaultOptions())
	p.RegisterType("container", &structs.Container{})
	p.RegisterType("network", &structs.Network{})
	p.RegisterType("template", &structs.Template{})

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	return c
}

func TestDoYaLikeDAGAddsDependencies(t *testing.T) {
	c := setupGraphConfig(t)

	g, err := doYaLikeDAGs(c)
	require.NoError(t, err)

	network, err := c.FindResource("resource.network.onprem")
	require.NoError(t, err)

	template, err := c.FindResource("resource.template.consul_config")
	require.NoError(t, err)

	// check the dependency tree of the base container
	base, err := c.FindResource("resource.container.base")
	require.NoError(t, err)

	s, err := g.Descendents(base)
	require.NoError(t, err)

	// check the network is returned
	list := s.List()
	require.Contains(t, list, network)

	// check the dependency tree of the consul container
	consul, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)

	s, err = g.Descendents(consul)
	require.NoError(t, err)

	// check the network is returned
	list = s.List()
	require.Contains(t, list, network)
	require.Contains(t, list, base)
	require.Contains(t, list, template)
}

func TestDependenciesValidNoError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/deps/valid.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)
}

func TestDependenciesInvalidError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/deps/invalid.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.Error(t, err)

	cfgErr, ok := err.(*errors.ConfigError)
	require.True(t, ok)

	require.Len(t, cfgErr.Errors, 12)
}
