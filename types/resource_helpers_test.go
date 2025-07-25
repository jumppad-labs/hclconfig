package types

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/hclconfig/internal/schema"
	"github.com/stretchr/testify/require"
)

type TestBaseResource struct {
	ResourceBase
	Name string
}

type TestExtendedResource struct {
	TestBaseResource
	ExtraField string
}

func testCreateBasicResource() *TestBaseResource {
	return &TestBaseResource{
		ResourceBase: ResourceBase{
			DependsOn: []string{"dependency1", "dependency2"},
			Disabled:  true,
			Meta: Meta{
				ID:   "test-id",
				Name: "test-name",
				Type: "test-type",
			},
		},
		Name: "test-resource",
	}
}

func testCreateExtendedResource() *TestExtendedResource {
	br := testCreateBasicResource()
	return &TestExtendedResource{
		TestBaseResource: *br,
		ExtraField:       "extra-value",
	}
}

func TestCanGetMetaOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "test-id", meta.ID)
	require.Equal(t, "test-name", meta.Name)
	require.Equal(t, "test-type", meta.Type)
}

func TestCanSetMetaOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	meta.ID = "new-id"
	meta.Name = "new-name"
	meta.Type = "new-type"

	newMeta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "new-id", newMeta.ID)
	require.Equal(t, "new-name", newMeta.Name)
	require.Equal(t, "new-type", newMeta.Type)
}

func TestCanGetMetaOnExtendedResource(t *testing.T) {
	te := testCreateExtendedResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "test-id", meta.ID)
	require.Equal(t, "test-name", meta.Name)
	require.Equal(t, "test-type", meta.Type)
}

func TestCanSetMetaOnExtendedResource(t *testing.T) {
	te := testCreateExtendedResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	meta.ID = "new-id"
	meta.Name = "new-name"
	meta.Type = "new-type"

	newMeta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "new-id", newMeta.ID)
	require.Equal(t, "new-name", newMeta.Name)
	require.Equal(t, "new-type", newMeta.Type)
}

func TestCanGetDependenciesOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	require.Len(t, deps, 2)
	require.Contains(t, deps, "dependency1")
	require.Contains(t, deps, "dependency2")
}

func TestCanSetDependenciesOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	deps = append(deps, "dependency3")

	err = SetDependencies(te, deps)
	require.NoError(t, err)

	newDeps, err := GetDependencies(te)
	require.NoError(t, err)
	require.Len(t, newDeps, 3)
	require.Contains(t, newDeps, "dependency1")
	require.Contains(t, newDeps, "dependency2")
	require.Contains(t, newDeps, "dependency3")
}

func TestCanSetUniqueDependencyOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	err := AppendUniqueDependency(te, "dependency3")
	require.NoError(t, err)
	err = AppendUniqueDependency(te, "dependency3")
	require.NoError(t, err)

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	require.Len(t, deps, 3) // Should not add duplicate
	require.Contains(t, deps, "dependency1")
	require.Contains(t, deps, "dependency2")
	require.Contains(t, deps, "dependency3")
}

func TestCanGetDependenciesOnExtendedResource(t *testing.T) {
	te := testCreateExtendedResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	require.Len(t, deps, 2)
	require.Contains(t, deps, "dependency1")
	require.Contains(t, deps, "dependency2")
}

func TestCanGetMetaOnExtendedResourceWhenCreatedFromSchema(t *testing.T) {
	te := testCreateExtendedResource()

	sch, err := schema.GenerateSchemaFromInstance(te, 10)
	require.NoError(t, err)

	typeMapping := map[string]reflect.Type{
		"types.Meta":         reflect.TypeOf(Meta{}),
		"types.ResourceBase": reflect.TypeOf(ResourceBase{}),
	}

	ni, err := schema.CreateInstanceFromSchema(sch, typeMapping)
	require.NoError(t, err)

	// The schema doesn't preserve values, only structure.
	// This test should verify that:
	// 1. GetMeta works on the newly created instance (returns no error)
	// 2. The Meta field is of the correct type (types.Meta)
	// 3. We can set and get values on the Meta field

	// Step 1: Verify GetMeta works without error
	meta, err := GetMeta(ni)
	require.NoError(t, err)
	require.NotNil(t, meta)

	// Step 2: Verify the Meta is of the correct type
	require.IsType(t, &Meta{}, meta)

	// Step 3: Verify we can set and get values
	meta.ID = "new-id"
	meta.Name = "new-name"
	meta.Type = "new-type"

	// Get meta again to verify the values were set
	meta2, err := GetMeta(ni)
	require.NoError(t, err)
	require.Equal(t, "new-id", meta2.ID)
	require.Equal(t, "new-name", meta2.Name)
	require.Equal(t, "new-type", meta2.Type)
}

func TestCanSetDependenciesOnExtendedResource(t *testing.T) {
	te := testCreateBasicResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	deps = append(deps, "dependency3")

	err = SetDependencies(te, deps)
	require.NoError(t, err)

	newDeps, err := GetDependencies(te)
	require.NoError(t, err)
	require.Len(t, newDeps, 3)
	require.Contains(t, newDeps, "dependency1")
	require.Contains(t, newDeps, "dependency2")
	require.Contains(t, newDeps, "dependency3")
}

func TestCanGetDisabledOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	d, err := GetDisabled(te)
	require.NoError(t, err)

	require.True(t, d)
}

func TestCanSetDisabledOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	err := SetDisabled(te, false)
	require.NoError(t, err)

	d, err := GetDisabled(te)
	require.NoError(t, err)
	require.False(t, d)
}

func TestCanGetDisabledOnExtendedResource(t *testing.T) {
	te := testCreateBasicResource()

	d, err := GetDisabled(te)
	require.NoError(t, err)

	require.True(t, d)
}

func TestCanSetDisabledOnExtendedResource(t *testing.T) {
	te := testCreateBasicResource()

	err := SetDisabled(te, false)
	require.NoError(t, err)

	d, err := GetDisabled(te)
	require.NoError(t, err)
	require.False(t, d)
}
