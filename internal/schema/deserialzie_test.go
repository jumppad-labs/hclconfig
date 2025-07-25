package schema

import (
	"reflect"
	"testing"

	fixtures "github.com/jumppad-labs/hclconfig/internal/schema/test_fixtures"
	"github.com/stretchr/testify/require"
)

func TestDeserializeInt(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyIntJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "int", getKindForField(s, 0, 0))
}

func TestDeserializeInt32(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyInt32JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "int32", getKindForField(s, 0, 0))
}

func TestDeserializeInt64(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyInt64JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "int64", getKindForField(s, 0, 0))
}

func TestDeserializeIntPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyIntPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "int", getKindForField(s, 0, 1))
}

func TestDeserializeIntSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyIntSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "int", getKindForField(s, 0, 1))
}

func TestDeserializeIntPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyIntPtrSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "int", getKindForField(s, 0, 2))
}

func TestDeserializeIntMapSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyIntMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "int", getKindForField(s, 0, 1))
}

func TestDeserializeIntMapPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyIntMapPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "int", getKindForField(s, 0, 2))
}

func TestDeserializeUInt(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "uint", getKindForField(s, 0, 0))
}

func TestDeserializeUIntPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "uint", getKindForField(s, 0, 1))
}

func TestDeserializeUInt32(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUInt32JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "uint32", getKindForField(s, 0, 0))
}

func TestDeserializeUInt64(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUInt64JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "uint64", getKindForField(s, 0, 0))
}

func TestDeserializeUIntSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "uint", getKindForField(s, 0, 1))
}

func TestDeserializeUIntPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntPtrSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "uint", getKindForField(s, 0, 2))
}

func TestDeserializeUIntMapSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "uint", getKindForField(s, 0, 1))
}

func TestDeserializeUIntMapPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntMapPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "uint", getKindForField(s, 0, 2))
}

func TestDeserializeFloat32(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloat32JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "float32", getKindForField(s, 0, 0))
}

func TestDeserializeFloat64(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloat64JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "float64", getKindForField(s, 0, 0))
}

func TestDeserializeFloatPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloatPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "float64", getKindForField(s, 0, 1))
}

func TestDeserializeFloatSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloatSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "float64", getKindForField(s, 0, 1))
}

func TestDeserializeFloatPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloatPtrSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "float64", getKindForField(s, 0, 2))
}

func TestDeserializeFloatMapSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloatMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "float64", getKindForField(s, 0, 1))
}

func TestDeserializeFloatMapPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyFloatMapPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "float64", getKindForField(s, 0, 2))
}

func TestDeserializeComplex64(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplex64JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "complex64", getKindForField(s, 0, 0))
}

func TestDeserializeComplex128(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplex128JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "complex128", getKindForField(s, 0, 0))
}

func TestDeserializeComplexPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplexPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "complex64", getKindForField(s, 0, 1))
}

func TestDeserializeComplexSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplexSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "complex64", getKindForField(s, 0, 1))
}

func TestDeserializeComplexPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplexPtrSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "complex64", getKindForField(s, 0, 2))
}

func TestDeserializeComplexMapSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplexMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "complex64", getKindForField(s, 0, 1))
}

func TestDeserializeComplexMapPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyComplexMapPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "complex64", getKindForField(s, 0, 2))
}

func TestDeserializeString(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStringJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "string", getKindForField(s, 0, 0))
}

func TestDeserializeStringPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStringPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "string", getKindForField(s, 0, 1))
}

func TestDeserializeStringSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStringSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "string", getKindForField(s, 0, 1))
}

func TestDeserializeStringPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStringPtrSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "string", getKindForField(s, 0, 2))
}

func TestDeserializeStringMapSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStringMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "string", getKindForField(s, 0, 1))
}

func TestDeserializeStringMapPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStringMapPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "string", getKindForField(s, 0, 2))
}

func TestDeserializeUIntPtrT(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyUIntPtrTJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "uintptr", getKindForField(s, 0, 0))
}

func TestDeserializeBool(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyBoolJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "bool", getKindForField(s, 0, 0))
}

func TestDeserializeBoolPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyBoolPtrJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "bool", getKindForField(s, 0, 1))
}

func TestDeserializeBoolSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyBoolSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "bool", getKindForField(s, 0, 1))
}

func TestDeserializeBoolPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyBoolPtrSliceJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "bool", getKindForField(s, 0, 2))
}

func TestDeserializeBoolMap(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyBoolMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "bool", getKindForField(s, 0, 1))
}

func TestDeserializeBoolPtrMap(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyBoolPtrMapJSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "bool", getKindForField(s, 0, 2))
}

func TestDeserializeStructPtr(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStructPtrDepth2JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 1, 0))
	require.Equal(t, "struct", getKindForField(s, 1, 1))

	// get the child field type from the data struct
	dataValue := reflect.ValueOf(s).Elem()
	structField, found := dataValue.Type().FieldByName("Struct")
	require.True(t, found)
	require.True(t, structField.Type.Kind() == reflect.Ptr)

	// Check that the pointer type points to a struct with the correct fields
	elemType := structField.Type.Elem()
	require.Equal(t, reflect.Struct, elemType.Kind())
	require.Equal(t, 1, elemType.NumField()) // Only Name field

	// Verify the Name field (field 0) is a string
	nameField := elemType.Field(0)
	require.Equal(t, "Name", nameField.Name)
	require.Equal(t, reflect.String, nameField.Type.Kind())
}

func TestDeserializeStructSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStructSliceDepth2JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 1, 0))
	require.Equal(t, "struct", getKindForField(s, 1, 1))
}

func TestDeserializeStructPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStructPtrSliceDepth2JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 1, 0))
	require.Equal(t, "ptr", getKindForField(s, 1, 1))
	require.Equal(t, "struct", getKindForField(s, 1, 2))
}

func TestDeserializeStructMapSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStructMapDepth2JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 1, 0))
	require.Equal(t, "struct", getKindForField(s, 1, 1))
}

func TestDeserializeStructMapPtrSlice(t *testing.T) {

	s, err := CreateInstanceFromSchema([]byte(fixtures.MyStructMapPtrDepth2JSON), nil)
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 1, 0))
	require.Equal(t, "ptr", getKindForField(s, 1, 1))
	require.Equal(t, "struct", getKindForField(s, 1, 2))
}

func TestDeserializeEmbeddedStruct(t *testing.T) {
	s, err := CreateInstanceFromSchema([]byte(fixtures.EmbeddedJson), nil)
	require.NoError(t, err)

	// Add proper assertions instead of t.Fail()
	require.NotNil(t, s)

	structType := reflect.TypeOf(s).Elem()

	// should have 2 fields: ResourceBase and Name
	require.Equal(t, 2, structType.NumField()) // ResourceBase + Name

	// Verify Name field exists
	_, found := structType.FieldByName("Name")
	require.True(t, found)

	// Verify ResourceBase field exists
	_, found = structType.FieldByName("ResourceBase")
	require.True(t, found)

}

func TestDeserializeEmbeddedStructContainingChild(t *testing.T) {
	s, err := CreateInstanceFromSchema([]byte(fixtures.EmbeddedInEmbeddedJson), nil)
	require.NoError(t, err)

	structType := reflect.TypeOf(s).Elem()

	// we expect the struct to have 2 fields the embedded type and the ChildName
	require.Equal(t, 2, structType.NumField()) // ResourceBase + Name

	_, found := structType.FieldByName("ChildName")
	require.True(t, found)

	// Verify the Embedded field exists
	_, found = structType.FieldByName("Embedded")
	require.True(t, found)

	// Verify ResourceBase field exists
	_, found = structType.FieldByName("ResourceBase")
	require.True(t, found)
}

func getKindForField(t any, field, depth int) string {
	// Now that CreateInstanceFromSchema returns any (the actual struct),
	// we can work directly with the struct
	actualType := reflect.TypeOf(t).Elem()

	// No longer need to skip ResourceBase - use field index directly
	fieldIndex := field
	if fieldIndex >= actualType.NumField() {
		return "interface" // fallback
	}

	fieldType := actualType.Field(fieldIndex).Type

	switch depth {
	case 0:
		return fieldType.Kind().String()
	case 1:
		return fieldType.Elem().Kind().String()
	case 2:
		return fieldType.Elem().Elem().Kind().String()
	}

	return ""
}
