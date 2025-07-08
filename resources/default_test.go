package resources

import (
	"reflect"
	"testing"

	"github.com/instruqt/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func TestCreateResouceReturnsNotRegisteredError(t *testing.T) {
	rt := types.RegisteredTypes{}
	_, err := rt.CreateResource("foo", "bar")
	require.Error(t, err)
}

func TestDefaultTypes(t *testing.T) {
	dt := DefaultResources()

	require.Equal(t, reflect.TypeOf(dt["variable"]), reflect.TypeOf(&Variable{}))
	require.Equal(t, reflect.TypeOf(dt["local"]), reflect.TypeOf(&Local{}))
	require.Equal(t, reflect.TypeOf(dt["module"]), reflect.TypeOf(&Module{}))
	require.Equal(t, reflect.TypeOf(dt["output"]), reflect.TypeOf(&Output{}))
}

func TestCreateResourceCreatesType(t *testing.T) {
	dt := DefaultResources()

	r, e := dt.CreateResource(TypeVariable, "test")
	require.NoError(t, e)
	require.NotNil(t, r)

	require.Equal(t, r.Metadata().Type, TypeVariable)
	require.Equal(t, r.Metadata().Name, "test")
}
