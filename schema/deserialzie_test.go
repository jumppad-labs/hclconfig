package schema

import (
	"reflect"
	"testing"

	fixtures "github.com/jumppad-labs/hclconfig/schema/test_fixtures"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestDeserializeInt(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntJSON))
	require.NoError(t, err)

	require.Equal(t, "int", getKindForField(s, 0))
}

func TestDeserializeIntPtr(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyIntPtrJSON))
	require.NoError(t, err)
	pretty.Print(s)
	require.Equal(t, "int", getKindForField(s, 0))
	require.False(t, isFieldPtr(s, 0))
}

func TestDeserializeInt32(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyInt32JSON))
	require.NoError(t, err)

	require.Equal(t, "int32", getKindForField(s, 0))
}

func TestDeserializeInt64(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyInt64JSON))
	require.NoError(t, err)

	require.Equal(t, "int64", getKindForField(s, 0))
}

func TestDeserializeString(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyStringJSON))
	require.NoError(t, err)

	require.Equal(t, "string", getKindForField(s, 0))
}

func TestDeserializeBool(t *testing.T) {

	s, err := CreateStructFromSchema([]byte(fixtures.MyBoolJSON))
	require.NoError(t, err)

	require.Equal(t, "bool", getKindForField(s, 0))
}

func getKindForField(t any, field int) string {
	return reflect.TypeOf(t).Elem().Field(field).Type.Kind().String()
}

func isFieldPtr(t any, field int) bool {
	return reflect.TypeOf(t).Elem().Field(field).IsExported()
}
