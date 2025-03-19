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

func TestDeserializeBool(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolJSON))
	require.NoError(t, err)

	require.Equal(t, "bool", getKindForField(s, 0, 0))
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
