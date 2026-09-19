package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// Thing is a plain Go resource type used to test type registration
type Thing struct {
	types.ResourceBase `xcl:",remain"`

	Size int `xcl:"size" json:"size"`
}

// Gadget is a second plain Go resource type, used to check that a clash
// leaves the first registered type in place
type Gadget struct {
	types.ResourceBase `xcl:",remain"`

	Colour string `xcl:"colour" json:"colour"`
}

// ComputedThing is a resource type with a computed field
type ComputedThing struct {
	types.ResourceBase `xcl:",remain"`

	Size int `xcl:"size" json:"size"`

	// ProviderID is a computed field
	ProviderID string `xcl:"provider_id,optional,computed" json:"provider_id,omitempty"`
}

// NotAResource is a struct that does not embed types.ResourceBase
type NotAResource struct {
	Size int `xcl:"size" json:"size"`
}

// thingProvider is a no-op provider for Thing resources
type thingProvider struct {
	plugins.DefaultChanged[*Thing]
}

var _ plugins.ResourceProvider[*Thing] = (*thingProvider)(nil)

func (p *thingProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *thingProvider) Create(ctx context.Context, resource *Thing) (*Thing, error) {
	return resource, nil
}

func (p *thingProvider) Destroy(ctx context.Context, resource *Thing, force bool) error {
	return nil
}

func (p *thingProvider) Read(ctx context.Context, old *Thing, new *Thing) (*Thing, error) {
	return new, nil
}

func (p *thingProvider) Update(ctx context.Context, resource *Thing) (*Thing, error) {
	return resource, nil
}

func (p *thingProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// thingPlugin is an in-process plugin that provides the resource type "thing"
type thingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*thingPlugin)(nil)

func (p *thingPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
}

// doubleThingPlugin is an in-process plugin that reports the resource type
// "thing" twice
type doubleThingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*doubleThingPlugin)(nil)

func (p *doubleThingPlugin) Init(logger logger.Logger, state plugins.State) error {
	err := plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
}

func TestRegisterTypeSucceedsWithoutPlugins(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	require.Empty(t, r.GetPluginHosts())
}

func TestCreateResourceReturnsRegisteredGoType(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	resource, err := r.CreateResource("thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, "thing", thing.Meta.Type)
}

func TestRegisterTypeRejectsDuplicateName(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	err = r.RegisterType("thing", &Gadget{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "thing", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	resource, err := r.CreateResource("thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, "thing", thing.Meta.Type)
}

func TestRegisterTypeRejectsBuiltinNameVariable(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("variable", &Thing{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"variable"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "variable", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("variable"))
}

func TestRegisterTypeRejectsBuiltinNameOutput(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("output", &Thing{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"output"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "output", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("output"))
}

func TestRegisterTypeRejectsBuiltinNameModule(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("module", &Thing{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"module"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "module", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("module"))
}

func TestRegisterTypeRejectsBuiltinNameRoot(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("root", &Thing{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"root"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "root", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("root"))
}

func TestRegisterTypeRejectsPluginProvidedName(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.RegisterType("thing", &Thing{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "thing", clash.Name)
	require.Equal(t, "plugin", clash.Existing)
	require.False(t, r.IsRegisteredType("thing"))
}

func TestRegisterPluginRejectsRegisteredTypeName(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	err = r.RegisterPlugin(&thingPlugin{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "thing", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
}

func TestRegisterPluginRejectsNameFromAnotherPlugin(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.RegisterPlugin(&thingPlugin{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "thing", clash.Name)
	require.Equal(t, "plugin", clash.Existing)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestRegisterPluginRejectsPluginReportingSameTypeTwice(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterPlugin(&doubleThingPlugin{})
	require.Error(t, err)
	require.ErrorContains(t, err, `"thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "thing", clash.Name)
	require.Equal(t, "the same plugin", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
}

func TestRegisterPluginAcceptsPluginWithUniqueTypes(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("gadget", &Gadget{})
	require.NoError(t, err)

	err = r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestRegisterTypeAcceptsComputedField(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("computed_thing", &ComputedThing{})
	require.NoError(t, err)

	resource, err := r.CreateResource("computed_thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*ComputedThing)
	require.True(t, ok, "expected *ComputedThing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, "computed_thing", thing.Meta.Type)
}

func TestRegisterTypeRejectsNonPointer(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", Thing{})
	require.Error(t, err)
	require.ErrorContains(t, err, `type "thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("thing"))
}

func TestRegisterTypeRejectsNil(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `type "thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("thing"))
}

func TestRegisterTypeRejectsNilPointer(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	var thing *Thing
	err := r.RegisterType("thing", thing)
	require.Error(t, err)
	require.ErrorContains(t, err, `type "thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("thing"))
}

func TestRegisterTypeRejectsPointerToNonStruct(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	size := 1
	err := r.RegisterType("thing", &size)
	require.Error(t, err)
	require.ErrorContains(t, err, `type "thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("thing"))
}

func TestRegisterTypeRejectsTypeWithoutResourceBase(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", &NotAResource{})
	require.Error(t, err)
	require.ErrorContains(t, err, `type "thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("thing"))
}

func TestIsRegisteredTypeReportsRegisteredTypes(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	require.True(t, r.IsRegisteredType("thing"))
}

func TestIsRegisteredTypeIgnoresUnknownTypes(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	require.False(t, r.IsRegisteredType("thing"))
}

func TestIsRegisteredTypeIgnoresBuiltinTypes(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	require.False(t, r.IsRegisteredType("variable"))
}

func TestIsRegisteredTypeIgnoresPluginTypes(t *testing.T) {
	r := NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	require.False(t, r.IsRegisteredType("thing"))
}

func TestTypeNameClashErrorMessage(t *testing.T) {
	err := &TypeNameClashError{Name: "thing", Existing: "plugin"}

	require.Equal(t, `type "thing" is already provided by plugin`, err.Error())
}
