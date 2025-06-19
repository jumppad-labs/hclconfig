package schema

import (
	"reflect"
	"testing"

	fixtures "github.com/jumppad-labs/hclconfig/schema/test_fixtures"
	"github.com/stretchr/testify/require"
)

func TestDeserializeInt(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntJSON))
	require.NoError(t, err)

	require.Equal(t, "int", getKindForField(s, 0, 0))
}

func TestDeserializeInt32(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyInt32JSON))
	require.NoError(t, err)

	require.Equal(t, "int32", getKindForField(s, 0, 0))
}

func TestDeserializeInt64(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyInt64JSON))
	require.NoError(t, err)

	require.Equal(t, "int64", getKindForField(s, 0, 0))
}

func TestDeserializeIntPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "int", getKindForField(s, 0, 1))
}

func TestDeserializeIntSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "int", getKindForField(s, 0, 1))
}

func TestDeserializeIntPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntPtrSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "int", getKindForField(s, 0, 2))
}

func TestDeserializeIntMapSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "int", getKindForField(s, 0, 1))
}

func TestDeserializeIntMapPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntMapPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "int", getKindForField(s, 0, 2))
}

func TestDeserializeUInt(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntJSON))
	require.NoError(t, err)

	require.Equal(t, "uint", getKindForField(s, 0, 0))
}

func TestDeserializeUIntPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "uint", getKindForField(s, 0, 1))
}

func TestDeserializeUInt32(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUInt32JSON))
	require.NoError(t, err)

	require.Equal(t, "uint32", getKindForField(s, 0, 0))
}

func TestDeserializeUInt64(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUInt64JSON))
	require.NoError(t, err)

	require.Equal(t, "uint64", getKindForField(s, 0, 0))
}

func TestDeserializeUIntSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "uint", getKindForField(s, 0, 1))
}

func TestDeserializeUIntPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntPtrSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "uint", getKindForField(s, 0, 2))
}

func TestDeserializeUIntMapSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "uint", getKindForField(s, 0, 1))
}

func TestDeserializeUIntMapPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntMapPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "uint", getKindForField(s, 0, 2))
}

func TestDeserializeFloat32(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloat32JSON))
	require.NoError(t, err)

	require.Equal(t, "float32", getKindForField(s, 0, 0))
}

func TestDeserializeFloat64(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloat64JSON))
	require.NoError(t, err)

	require.Equal(t, "float64", getKindForField(s, 0, 0))
}

func TestDeserializeFloatPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloatPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "float64", getKindForField(s, 0, 1))
}

func TestDeserializeFloatSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloatSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "float64", getKindForField(s, 0, 1))
}

func TestDeserializeFloatPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloatPtrSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "float64", getKindForField(s, 0, 2))
}

func TestDeserializeFloatMapSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloatMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "float64", getKindForField(s, 0, 1))
}

func TestDeserializeFloatMapPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyFloatMapPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "float64", getKindForField(s, 0, 2))
}

func TestDeserializeComplex64(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplex64JSON))
	require.NoError(t, err)

	require.Equal(t, "complex64", getKindForField(s, 0, 0))
}

func TestDeserializeComplex128(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplex128JSON))
	require.NoError(t, err)

	require.Equal(t, "complex128", getKindForField(s, 0, 0))
}

func TestDeserializeComplexPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplexPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "complex64", getKindForField(s, 0, 1))
}

func TestDeserializeComplexSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplexSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "complex64", getKindForField(s, 0, 1))
}

func TestDeserializeComplexPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplexPtrSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "complex64", getKindForField(s, 0, 2))
}

func TestDeserializeComplexMapSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplexMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "complex64", getKindForField(s, 0, 1))
}

func TestDeserializeComplexMapPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyComplexMapPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "complex64", getKindForField(s, 0, 2))
}

func TestDeserializeString(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringJSON))
	require.NoError(t, err)

	require.Equal(t, "string", getKindForField(s, 0, 0))
}

func TestDeserializeStringPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "string", getKindForField(s, 0, 1))
}

func TestDeserializeStringSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "string", getKindForField(s, 0, 1))
}

func TestDeserializeStringPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringPtrSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "string", getKindForField(s, 0, 2))
}

func TestDeserializeStringMapSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "string", getKindForField(s, 0, 1))
}

func TestDeserializeStringMapPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringMapPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "string", getKindForField(s, 0, 2))
}

func TestDeserializeUIntPtrT(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyUIntPtrTJSON))
	require.NoError(t, err)

	require.Equal(t, "uintptr", getKindForField(s, 0, 0))
}

func TestDeserializeBool(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolJSON))
	require.NoError(t, err)

	require.Equal(t, "bool", getKindForField(s, 0, 0))
}

func TestDeserializeBoolPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolPtrJSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 0, 0))
	require.Equal(t, "bool", getKindForField(s, 0, 1))
}

func TestDeserializeBoolSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "bool", getKindForField(s, 0, 1))
}

func TestDeserializeBoolPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolPtrSliceJSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "bool", getKindForField(s, 0, 2))
}

func TestDeserializeBoolMap(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "bool", getKindForField(s, 0, 1))
}

func TestDeserializeBoolPtrMap(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolPtrMapJSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 0, 0))
	require.Equal(t, "ptr", getKindForField(s, 0, 1))
	require.Equal(t, "bool", getKindForField(s, 0, 2))
}

func TestDeserializeStructPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStructPtrDepth2JSON))
	require.NoError(t, err)

	require.Equal(t, "ptr", getKindForField(s, 1, 0))
	require.Equal(t, "struct", getKindForField(s, 1, 1))

	// get the child
	co := reflect.ValueOf(s).Elem().FieldByName("Struct").Interface()
	require.Equal(t, "string", getKindForField(co, 0, 0))
}

func TestDeserializeStructSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStructSliceDepth2JSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 1, 0))
	require.Equal(t, "struct", getKindForField(s, 1, 1))
}

func TestDeserializeStructPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStructPtrSliceDepth2JSON))
	require.NoError(t, err)

	require.Equal(t, "slice", getKindForField(s, 1, 0))
	require.Equal(t, "ptr", getKindForField(s, 1, 1))
	require.Equal(t, "struct", getKindForField(s, 1, 2))
}

func TestDeserializeStructMapSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStructMapDepth2JSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 1, 0))
	require.Equal(t, "struct", getKindForField(s, 1, 1))
}

func TestDeserializeStructMapPtrSlice(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStructMapPtrDepth2JSON))
	require.NoError(t, err)

	require.Equal(t, "map", getKindForField(s, 1, 0))
	require.Equal(t, "ptr", getKindForField(s, 1, 1))
	require.Equal(t, "struct", getKindForField(s, 1, 2))
}

func TestDeserializeEmbeddedStruct(t *testing.T) {
	s, err := CreateStructFromSchema([]byte(fixtures.EmbeddedJson))
	require.NoError(t, err)
	
	// Add proper assertions instead of t.Fail()
	require.NotNil(t, s)
	
	// Verify the struct has the expected fields
	structType := reflect.TypeOf(s).Elem()
	require.Equal(t, 2, structType.NumField()) // ResourceBase + Name
	
	// Verify ResourceBase field exists
	_, found := structType.FieldByName("ResourceBase")
	require.True(t, found)
	
	// Verify Name field exists  
	_, found = structType.FieldByName("Name")
	require.True(t, found)
}

func getKindForField(t any, field, depth int) string {
	switch depth {
	case 0:
		return reflect.TypeOf(t).Elem().Field(field).Type.Kind().String()
	case 1:
		return reflect.TypeOf(t).Elem().Field(field).Type.Elem().Kind().String()
	case 2:
		return reflect.TypeOf(t).Elem().Field(field).Type.Elem().Elem().Kind().String()
	}

	return ""
}
