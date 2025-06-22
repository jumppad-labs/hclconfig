package hclconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupGraphConfig(t *testing.T) *Config {
	absoluteFolderPath, err := filepath.Abs("./internal/test_fixtures/config/simple/container.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupTestParser()

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
