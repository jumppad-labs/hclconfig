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

	require.Equal(t, "int", getKindForField(s, 0))
}

func getKindForField(t any, field int) string {
	return reflect.TypeOf(t).Elem().Field(field).Type.Kind().String()
}
