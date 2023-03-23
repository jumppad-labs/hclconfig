package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var typeTestContainer = "container"

type testContainer struct {
	// embedded type holding name, etc
	ResourceMetadata `hcl:",remain"`
}

func TestParseFQDNParsesComponents(t *testing.T) {
	fqdn, err := ParseFQDN("module.module1.module2.resource.container.mine.attr")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqdn.Module)
	require.Equal(t, typeTestContainer, fqdn.Type)
	require.Equal(t, "mine", fqdn.Resource)
	require.Equal(t, "attr", fqdn.Attribute)
}

func TestParseFQDNReturnsErrorOnMissingType(t *testing.T) {
	_, err := ParseFQDN("module.module1.module2.resource.mine")
	require.Error(t, err)
}

func TestParseFQDNReturnsErrorOnNoModuleOrResource(t *testing.T) {
	_, err := ParseFQDN("module1.module2")
	require.Error(t, err)
}

func TestParseFQDNReturnsModuleWhenNoResource(t *testing.T) {
	fqdn, err := ParseFQDN("module.module1.module2")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqdn.Module)
}

func TestParseFQDNReturnsModuleWhenOutput(t *testing.T) {
	fqdn, err := ParseFQDN("module.module1.module2.output.mine")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqdn.Module)
	require.Equal(t, TypeOutput, fqdn.Type)
	require.Equal(t, "mine", fqdn.Resource)
	require.Equal(t, "value", fqdn.Attribute)
}

func TestFQDNStringWithoutModuleReturnsCorrectly(t *testing.T) {
	fqdn, err := ParseFQDN("resource.container.mine")
	require.NoError(t, err)

	fqdnStr := fqdn.String()

	require.Equal(t, "resource.container.mine", fqdnStr)
}

func TestFQDNStringWithModuleOutputReturnsCorrectly(t *testing.T) {
	fqdn, err := ParseFQDN("module.module1.module2.output.mine")
	require.NoError(t, err)

	fqdnStr := fqdn.String()

	require.Equal(t, "module.module1.module2.output.mine", fqdnStr)
}

func TestFQDNStringWithModuleResourceReturnsCorrectly(t *testing.T) {
	fqdn, err := ParseFQDN("module.module1.module2.resource.container.mine")
	require.NoError(t, err)

	fqdnStr := fqdn.String()

	require.Equal(t, "module.module1.module2.resource.container.mine", fqdnStr)
}

func TestFQDNFromResouceReturnsCorrectData(t *testing.T) {
	dt := DefaultTypes()
	dt[typeTestContainer] = &testContainer{}

	r,err := dt.CreateResource(typeTestContainer, "mytest")
	require.NoError(t,err)

	r.Metadata().Module = "mymodule"
	
	fqdn := FQDNFromResource(r)
	
	require.Equal(t, "mymodule", fqdn.Module)
	require.Equal(t, "mytest", fqdn.Resource)
	require.Equal(t, typeTestContainer, fqdn.Type)
}
