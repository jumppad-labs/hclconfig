package parser

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/stretchr/testify/require"
)

// newTestRegistry returns a plugin registry with the test plugin registered, so
// resources it creates have the schema-rebuilt types the parser works with
func newTestRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	r := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := r.RegisterPlugin(&TestPlugin{})
	require.NoError(t, err)

	return r
}

// computedPaths returns the sorted paths of the computed fields of t
func computedPaths(t reflect.Type) []string {
	paths := []string{}
	for _, f := range computedFields(t) {
		paths = append(paths, f.path)
	}

	sort.Strings(paths)

	return paths
}

// unkeyedPort is a list element type with no key fields, its elements are
// paired by position
type unkeyedPort struct {
	Name     string `hcl:"name" json:"name"`
	Assigned string `hcl:"assigned,optional" json:"assigned,omitempty" xcl:"computed"`
}

// unkeyedHolder holds a list of blocks whose element type has no key fields
type unkeyedHolder struct {
	Ports []unkeyedPort `hcl:"port,block" json:"ports,omitempty"`
}

func TestComputedFieldsFindsTaggedFieldsOnSchemaRebuiltType(t *testing.T) {
	r := newTestRegistry(t)

	network, err := r.CreateResource("network", "x")
	require.NoError(t, err)

	// the registry rebuilds the type from the plugin's schema, it is not the
	// concrete fixture struct
	require.NotEqual(t, reflect.TypeOf(&structs.Network{}), reflect.TypeOf(network))

	paths := computedPaths(reflect.TypeOf(network))
	require.Equal(t, []string{"observed", "provider_id"}, paths)
}

func TestComputedFieldsFindsTaggedFieldsOnConcreteType(t *testing.T) {
	paths := computedPaths(reflect.TypeOf(structs.Network{}))
	require.Equal(t, []string{"observed", "provider_id"}, paths)
}

func TestComputedFieldsIncludesPromotedEmbeddedFields(t *testing.T) {
	// Container embeds ContainerBase, whose fields are promoted into the
	// container's configuration
	paths := computedPaths(reflect.TypeOf(&structs.Container{}))

	require.Equal(t, []string{
		"build.image_id",
		"created_network.assigned_address",
		"created_network_map.observed",
		"created_network_map.provider_id",
		"network.assigned_address",
		"networkobj.observed",
		"networkobj.provider_id",
	}, paths)
}

func TestComputedFieldsIncludesPromotedEmbeddedFieldsOnSchemaRebuiltType(t *testing.T) {
	r := newTestRegistry(t)

	container, err := r.CreateResource("container", "x")
	require.NoError(t, err)

	paths := computedPaths(reflect.TypeOf(container))

	require.Equal(t, []string{
		"build.image_id",
		"created_network.assigned_address",
		"created_network_map.observed",
		"created_network_map.provider_id",
		"network.assigned_address",
		"networkobj.observed",
		"networkobj.provider_id",
	}, paths)
}

func TestComputedFieldsFindsNestedBlockFields(t *testing.T) {
	found := computedFields(reflect.TypeOf(structs.ContainerBase{}))

	byPath := map[string]reflect.StructField{}
	for _, f := range found {
		byPath[f.path] = f.field
	}

	// a nested path names the block, then the field inside it
	require.Contains(t, byPath, "network.assigned_address")
	require.Equal(t, "AssignedAddress", byPath["network.assigned_address"].Name)

	require.Contains(t, byPath, "build.image_id")
	require.Equal(t, "ImageID", byPath["build.image_id"].Name)
}

func TestComputedFieldsReturnsNothingForTypeWithoutComputedFields(t *testing.T) {
	paths := computedPaths(reflect.TypeOf(structs.Volume{}))
	require.Empty(t, paths)
}

func TestCopyComputedCopiesOnlyComputedValues(t *testing.T) {
	src := structs.Network{Subnet: "10.0.0.0/16", ProviderID: "p-1", Observed: "seen"}
	dst := structs.Network{Subnet: "10.1.0.0/16"}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "p-1", dst.ProviderID)
	require.Equal(t, "seen", dst.Observed)
	require.Equal(t, "10.1.0.0/16", dst.Subnet)
}

func TestCopyComputedCopiesOnlyComputedValuesOnSchemaRebuiltType(t *testing.T) {
	r := newTestRegistry(t)

	src, err := r.CreateResource("network", "x")
	require.NoError(t, err)

	err = json.Unmarshal([]byte(`{"subnet":"10.0.0.0/16","provider_id":"p-1","observed":"seen"}`), src)
	require.NoError(t, err)

	dst, err := r.CreateResource("network", "x")
	require.NoError(t, err)

	err = json.Unmarshal([]byte(`{"subnet":"10.1.0.0/16"}`), dst)
	require.NoError(t, err)

	copyComputed(reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem())

	data, err := json.Marshal(dst)
	require.NoError(t, err)

	result := structs.Network{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Equal(t, "p-1", result.ProviderID)
	require.Equal(t, "seen", result.Observed)
	require.Equal(t, "10.1.0.0/16", result.Subnet)
}

func TestCopyComputedCopiesIntoPointerBlock(t *testing.T) {
	src := structs.Container{}
	src.Build = &structs.Build{Context: "./old", ImageID: "sha256:abc"}

	dst := structs.Container{}
	dst.Build = &structs.Build{Context: "./new"}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "sha256:abc", dst.Build.ImageID)
	require.Equal(t, "./new", dst.Build.Context)
}

func TestCopyComputedLeavesNilPointerBlockAlone(t *testing.T) {
	src := structs.Container{}
	src.Build = &structs.Build{Context: "./old", ImageID: "sha256:abc"}

	dst := structs.Container{}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Nil(t, dst.Build)
}

func TestCopyComputedLeavesPointerBlockAloneWhenSourceIsNil(t *testing.T) {
	src := structs.Container{}

	dst := structs.Container{}
	dst.Build = &structs.Build{Context: "./new", ImageID: "sha256:keep"}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.NotNil(t, dst.Build)
	require.Equal(t, "sha256:keep", dst.Build.ImageID)
}

func TestCopyComputedPairsListElementsByKey(t *testing.T) {
	src := structs.Container{}
	src.Networks = []structs.NetworkAttachment{
		{ID: 1, Name: "one", AssignedAddress: "10.0.0.1"},
		{ID: 2, Name: "two", AssignedAddress: "10.0.0.2"},
	}

	// the configured copy lists the same attachments in the opposite order
	dst := structs.Container{}
	dst.Networks = []structs.NetworkAttachment{
		{ID: 2, Name: "two"},
		{ID: 1, Name: "one"},
	}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, 2, dst.Networks[0].ID)
	require.Equal(t, "10.0.0.2", dst.Networks[0].AssignedAddress)

	require.Equal(t, 1, dst.Networks[1].ID)
	require.Equal(t, "10.0.0.1", dst.Networks[1].AssignedAddress)
}

func TestCopyComputedPairsListElementsByPositionWithoutKey(t *testing.T) {
	src := unkeyedHolder{Ports: []unkeyedPort{
		{Name: "a", Assigned: "first"},
		{Name: "b", Assigned: "second"},
	}}

	// without key fields elements pair by position, even when they differ
	dst := unkeyedHolder{Ports: []unkeyedPort{
		{Name: "b"},
		{Name: "a"},
	}}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "b", dst.Ports[0].Name)
	require.Equal(t, "first", dst.Ports[0].Assigned)

	require.Equal(t, "a", dst.Ports[1].Name)
	require.Equal(t, "second", dst.Ports[1].Assigned)
}

func TestCopyComputedLeavesExtraPositionalElementsAlone(t *testing.T) {
	src := unkeyedHolder{Ports: []unkeyedPort{
		{Name: "a", Assigned: "first"},
	}}

	dst := unkeyedHolder{Ports: []unkeyedPort{
		{Name: "a"},
		{Name: "b"},
	}}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "first", dst.Ports[0].Assigned)
	require.Equal(t, "", dst.Ports[1].Assigned)
}

func TestCopyComputedLeavesUnpairedElementsAlone(t *testing.T) {
	src := structs.Container{}
	src.Networks = []structs.NetworkAttachment{
		{ID: 1, Name: "one", AssignedAddress: "10.0.0.1"},
	}

	// attachment 3 is new, the saved copy has nothing to carry onto it
	dst := structs.Container{}
	dst.Networks = []structs.NetworkAttachment{
		{ID: 1, Name: "one"},
		{ID: 3, Name: "three"},
	}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "10.0.0.1", dst.Networks[0].AssignedAddress)
	require.Equal(t, "", dst.Networks[1].AssignedAddress)
}

func TestCopyComputedPairsMapElementsByKey(t *testing.T) {
	src := structs.Container{}
	src.CreatedNetworksMap = map[string]structs.Network{
		"a": {Subnet: "10.0.0.0/16", ProviderID: "p-a"},
	}

	dst := structs.Container{}
	dst.CreatedNetworksMap = map[string]structs.Network{
		"a": {Subnet: "10.1.0.0/16"},
		"b": {Subnet: "10.2.0.0/16"},
	}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "p-a", dst.CreatedNetworksMap["a"].ProviderID)
	require.Equal(t, "10.1.0.0/16", dst.CreatedNetworksMap["a"].Subnet)

	require.Equal(t, "", dst.CreatedNetworksMap["b"].ProviderID)
	require.Equal(t, "10.2.0.0/16", dst.CreatedNetworksMap["b"].Subnet)
}

func TestCopyComputedCopiesObjectField(t *testing.T) {
	src := structs.Container{}
	src.NetworkObj = structs.Network{Subnet: "10.0.0.0/16", ProviderID: "p-obj"}

	dst := structs.Container{}
	dst.NetworkObj = structs.Network{Subnet: "10.1.0.0/16"}

	copyComputed(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src))

	require.Equal(t, "p-obj", dst.NetworkObj.ProviderID)
	require.Equal(t, "10.1.0.0/16", dst.NetworkObj.Subnet)
}
