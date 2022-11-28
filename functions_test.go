package hclconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestCreateFunctionCreatesFunctionWithCorrectInParameters(t *testing.T) {
	myfunc := func(a string, b int) int {
		return 0
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	require.Equal(t, cty.String, ctyFunc.Params()[0].Type)
	require.Equal(t, cty.Number, ctyFunc.Params()[1].Type)
}

func TestCreateFunctionWithInvalidInParameterReturnsError(t *testing.T) {
	myfunc := func(a string, complex func() error) int {
		return 0
	}

	_, err := createCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)
}

func TestCreateFunctionCreatesFunctionWithCorrectOutParameters(t *testing.T) {
	myfunc := func(a string, b int) int {
		return 0
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	rt, err := ctyFunc.ReturnType([]cty.Type{cty.String, cty.Number})
	require.NoError(t, err)
	require.Equal(t, cty.Number, rt)
}

func TestCreateFunctionWithInvalidOutParameterReturnsError(t *testing.T) {
	myfunc := func(a string, b int) func() error {
		return func() error {
			return fmt.Errorf("oops")
		}
	}

	_, err := createCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)
}

func TestCreateFunctionCallsFunction(t *testing.T) {
	myfunc := func(a, b int) int {
		return a + b
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	val, err := ctyFunc.Call([]cty.Value{cty.NumberIntVal(2), cty.NumberIntVal(3)})
	require.NoError(t, err)

	bf := val.AsBigFloat()
	i, _ := bf.Int64()
	require.Equal(t, 5, i)
}
