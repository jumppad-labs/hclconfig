package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// parseFixture parses a fixture without decoding it, which is the point at
// which reference resolution runs, and returns the parser holding the
// populated working set.
func parseFixture(t *testing.T, path string) *Parser {
	t.Helper()

	p, _ := setupParser(t)

	_, _, err := p.parseAndValidate(path)
	require.NoError(t, err)

	return p
}

func TestResolveReferenceStripsTrailingAttributeBeforeLookup(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("resource.container.consul.network[0].ip_address", "")

	require.True(t, resolved.found)
	require.Equal(t, "resource.container.consul", resolved.key)
	require.Equal(t, "network[0].ip_address", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceSeparatesNestedAttributePath(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("resource.container.base.resources.cpu_pin", "")

	require.True(t, resolved.found)
	require.Equal(t, "resource.container.base", resolved.key)
	require.Equal(t, "resources.cpu_pin", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceWithoutAttributeResolvesWithEmptyAttribute(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("variable.cpu_resources", "")

	require.True(t, resolved.found)
	require.Equal(t, "variable.cpu_resources", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesResourceNamedOnItsOwn(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("resource.network.onprem", "")

	require.True(t, resolved.found)
	require.Equal(t, "resource.network.onprem", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesVariableForm(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("variable.default_cpu", "")

	require.True(t, resolved.found)
	require.Equal(t, "variable.default_cpu", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesOutputForm(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("module.consul_1.output.container_name", "")

	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1.output.container_name", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesUnqualifiedReferenceFromInsideItsModule(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("variable.cpu_resources", "consul_1")

	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1.variable.cpu_resources", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesUnqualifiedResourceWithAttributeFromInsideItsModule(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("resource.container.consul.resources.cpu", "consul_2")

	require.True(t, resolved.found)
	require.Equal(t, "module.consul_2.resource.container.consul", resolved.key)
	require.Equal(t, "resources.cpu", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesModuleOutputFromOutsideTheModule(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("module.consul_1.output.container_resources_cpu", "")

	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1.output.container_resources_cpu", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesModuleOutputAttributeFromOutsideTheModule(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("module.consul_1.output.combined_map.name", "")

	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1.output.combined_map", resolved.key)
	require.Equal(t, "name", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesModuleItself(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("module.consul_1", "")

	require.True(t, resolved.found)
	require.Equal(t, "module.consul_1", resolved.key)
	require.Equal(t, "", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	networkFile := filepath.Join(dir, "network.xcl")
	err := os.WriteFile(networkFile, []byte(`resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}
`), 0644)
	require.NoError(t, err)

	containerFile := filepath.Join(dir, "container.xcl")
	err = os.WriteFile(containerFile, []byte(`resource "container" "consul" {
  command = ["consul"]

  network {
    name       = resource.network.onprem.meta.name
    ip_address = "10.6.0.200"
  }
}
`), 0644)
	require.NoError(t, err)

	p := parseFixture(t, dir)

	// The reference lives in container.xcl while the resource it names is
	// declared in network.xcl.
	resolved := p.resolveReference("resource.network.onprem.meta.name", "")

	require.True(t, resolved.found)
	require.Equal(t, "resource.network.onprem", resolved.key)
	require.Equal(t, "meta.name", resolved.attribute)
	require.NotNil(t, resolved.target)
}

func TestResolveReferenceResolvesEveryLinkInSimpleFixture(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	links := 0

	for _, resource := range p.parsedResources.resources {
		meta, err := types.GetMeta(resource)
		require.NoError(t, err)

		for _, link := range meta.Links {
			links++

			resolved := p.resolveReference(link, meta.Module)
			require.True(t, resolved.found, "link %q from module %q did not resolve", link, meta.Module)
		}
	}

	require.Equal(t, 17, links)
}

func TestResolveReferenceResolvesEveryLinkInModulesFixture(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	links := 0

	for _, resource := range p.parsedResources.resources {
		meta, err := types.GetMeta(resource)
		require.NoError(t, err)

		for _, link := range meta.Links {
			links++

			resolved := p.resolveReference(link, meta.Module)
			require.True(t, resolved.found, "link %q from module %q did not resolve", link, meta.Module)
		}
	}

	require.Equal(t, 39, links)
}

func TestResolveReferenceResolvesEveryLinkInInterpolationFixture(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/interpolation/interpolation.xcl")

	links := 0

	for _, resource := range p.parsedResources.resources {
		meta, err := types.GetMeta(resource)
		require.NoError(t, err)

		for _, link := range meta.Links {
			links++

			resolved := p.resolveReference(link, meta.Module)
			require.True(t, resolved.found, "link %q from module %q did not resolve", link, meta.Module)
		}
	}

	require.Equal(t, 12, links)
}

func TestResolveReferenceResolvesEveryLinkInCyclicalPassFixture(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/cyclical/pass")

	links := 0

	for _, resource := range p.parsedResources.resources {
		meta, err := types.GetMeta(resource)
		require.NoError(t, err)

		for _, link := range meta.Links {
			links++

			resolved := p.resolveReference(link, meta.Module)
			require.True(t, resolved.found, "link %q from module %q did not resolve", link, meta.Module)
		}
	}

	require.Equal(t, 2, links)
}

func TestResolveReferenceDoesNotResolveUnknownResource(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("resource.container.does_not_exist", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveUnknownResourceWithAttribute(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("resource.container.does_not_exist.network[0].ip_address", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveUnknownVariable(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("variable.not_declared_anywhere", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveUnknownModuleOutput(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	resolved := p.resolveReference("module.consul_1.output.no_such_output", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveUnparseableReference(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("not a reference at all", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveEmptyReference(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/simple/container.xcl")

	resolved := p.resolveReference("", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveModuleScopedVariableWithoutModuleScope(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	// variable.cpu_resources is only ever declared inside the modules, keyed as
	// module.consul_1.variable.cpu_resources, so without the module scope there
	// is nothing at the global key for it to match.
	resolved := p.resolveReference("variable.cpu_resources", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveModuleScopedResourceWithoutModuleScope(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	// resource.container.consul is declared inside every module but never at the
	// top level, where only resource.container.base exists.
	resolved := p.resolveReference("resource.container.consul", "")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}

func TestResolveReferenceDoesNotResolveFromAModuleThatDoesNotDeclareIt(t *testing.T) {
	p := parseFixture(t, "../test_fixtures/config/modules/modules.xcl")

	// Neither module.consul_1.variable.not_declared_anywhere nor the global
	// variable.not_declared_anywhere exists, so the module fallback must not
	// invent a match.
	resolved := p.resolveReference("variable.not_declared_anywhere", "consul_1")

	require.False(t, resolved.found)
	require.Equal(t, "", resolved.key)
	require.Nil(t, resolved.target)
}
