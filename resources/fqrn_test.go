package resources

import (
	"testing"

	"github.com/instruqt/hclconfig/types"
	"github.com/stretchr/testify/require"
)

var typeTestContainer = "container"

type testContainer struct {
	// embedded type holding name, etc
	types.ResourceBase `hcl:",remain"`
}

func TestParseFQRNParsesComponents(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.module2.resource.container.mine.attr.other")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqrn.Module)
	require.Equal(t, typeTestContainer, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "attr.other", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.module2.resource.container.mine.attr.other", sfrqn)
}

func TestParseFQRNParsesComponents2(t *testing.T) {
	fqrn, err := ParseFQRN("module.consul_1.resource.network.onprem.name")
	require.NoError(t, err)

	require.Equal(t, "consul_1", fqrn.Module)
	require.Equal(t, "network", fqrn.Type)
	require.Equal(t, "onprem", fqrn.Resource)
	require.Equal(t, "name", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.consul_1.resource.network.onprem.name", sfrqn)
}

func TestParseFQRNParsesSplat(t *testing.T) {
	fqrn, err := ParseFQRN("resource.chapter.installation.tasks.*.id")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, "chapter", fqrn.Type)
	require.Equal(t, "installation", fqrn.Resource)
	require.Equal(t, "tasks.*.id", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "resource.chapter.installation.tasks.*.id", sfrqn)
}

func TestParseFQRNParsesModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeModule, fqrn.Type)
	require.Equal(t, "module1", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.module1", sfrqn)
}

func TestParseFQRNParsesModuleInModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.container")
	require.NoError(t, err)

	require.Equal(t, "module1", fqrn.Module)
	require.Equal(t, TypeModule, fqrn.Type)
	require.Equal(t, "container", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.container", sfrqn)
}

func TestParseFQDNReturnsErrorOnMissingType(t *testing.T) {
	_, err := ParseFQRN("module.module1.module2.resource.mine")
	require.Error(t, err)
}

func TestParseFQRNReturnsOutputWhenInNestedModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.module2.output.mine")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.module2.output.mine", sfrqn)
}

func TestParseFQRNReturnsOutputWhenInModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.consul_1.output.container_resources_cpu")
	require.NoError(t, err)

	require.Equal(t, "consul_1", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "container_resources_cpu", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.consul_1.output.container_resources_cpu", sfrqn)
}

func TestParseFQRNReturnsResource(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, "container", fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "resource.container.mine", sfrqn)
}

func TestParseFQRNReturnsResourceWithAttr(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine.my.stuff")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, "container", fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "my.stuff", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "resource.container.mine.my.stuff", sfrqn)
}

func TestParseFQRNReturnsOutputInModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	require.Equal(t, "module1", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.output.mine", sfrqn)
}

func TestParseFQRNReturnsOutput(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine", sfrqn)
}

func TestParseFQRNReturnsLocal(t *testing.T) {
	fqrn, err := ParseFQRN("local.mine")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeLocal, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "local.mine", sfrqn)
}

func TestParseResourceFQRNWithIndexReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine.property.0")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, typeTestContainer, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "property.0", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "resource.container.mine.property.0", sfrqn)
}

func TestParseFQRNWithIndexReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine.0")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "0", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine.0", sfrqn)
}

func TestParseFQRNWithParenthesesIndexReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine[0]")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "0", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine.0", sfrqn)
}

func TestParseFQRNWithParenthesesIndexAndAttributeReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine[0].nic")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "0.nic", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine.0.nic", sfrqn)
}

func TestParseFQRNWithIndexAndModuleReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("module.mine.output.mine.name")
	require.NoError(t, err)

	require.Equal(t, "mine", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "name", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.mine.output.mine.name", sfrqn)
}

func TestFQRNFromResourceReturnsCorrectData(t *testing.T) {
	dt := DefaultResources()
	dt[typeTestContainer] = &testContainer{}

	r, err := dt.CreateResource(typeTestContainer, "mytest")
	require.NoError(t, err)

	r.Metadata().Module = "mymodule"

	fqrn := FQRNFromResource(r)

	require.Equal(t, "mymodule", fqrn.Module)
	require.Equal(t, typeTestContainer, fqrn.Type)
	require.Equal(t, "mytest", fqrn.Resource)

	sfrqn := fqrn.String()
	require.Equal(t, "module.mymodule.resource.container.mytest", sfrqn)
}

func TestFQRNFromVariableReturnsCorrectData(t *testing.T) {
	dt := DefaultResources()

	r, err := dt.CreateResource(TypeVariable, "mytest")
	require.NoError(t, err)

	//r.Metadata().Module = "mymodule"

	fqrn := FQRNFromResource(r)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeVariable, fqrn.Type)
	require.Equal(t, "mytest", fqrn.Resource)

	sfrqn := fqrn.String()
	require.Equal(t, "variable.mytest", sfrqn)
}

func TestFQRNFromVariableInModuleReturnsCorrectData(t *testing.T) {
	dt := DefaultResources()

	r, err := dt.CreateResource(TypeVariable, "mytest")
	require.NoError(t, err)

	r.Metadata().Module = "mymodule"

	fqrn := FQRNFromResource(r)

	require.Equal(t, "mymodule", fqrn.Module)
	require.Equal(t, TypeVariable, fqrn.Type)
	require.Equal(t, "mytest", fqrn.Resource)

	sfrqn := fqrn.String()
	require.Equal(t, "module.mymodule.variable.mytest", sfrqn)
}

func TestFQRNAppendsParentCorrectlyWhenNoModule(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("parent")
	require.Equal(t, "module.parent.output.mine", new.String())
}

func TestFQRNAppendsParentCorrectlyWhenExistingModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("parent")
	require.Equal(t, "module.parent.module1.output.mine", new.String())
}

func TestFQRNAppedDoesNothingWhenNoParent(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("")
	require.Equal(t, "module.module1.output.mine", new.String())
}
