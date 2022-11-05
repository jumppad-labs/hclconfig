package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateResouceReturnsNotRegisteredError(t *testing.T) {
	rt := RegisteredTypes{}
	_, err := rt.CreateResource("foo", "bar")
	require.Error(t, err)
}

func TestDefaultTypes(t *testing.T) {
	dt := DefaultTypes()

	require.Equal(t, dt["variable"].Info().Type, TypeVariable)
	require.Equal(t, dt["module"].Info().Type, TypeModule)
	require.Equal(t, dt["output"].Info().Type, TypeOutput)
}

func TestCreateResourceCreatesType(t *testing.T) {
	dt := DefaultTypes()

	r, e := dt.CreateResource("variable", "test")
	require.NoError(t, e)
	require.NotNil(t, r)

	require.Equal(t, r.Info().Type, TypeVariable)
	require.Equal(t, r.Info().Name, "test")
}
