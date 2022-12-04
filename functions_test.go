package hclconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestCreateFunctionCreatesFunctionWithCorrectInParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	require.Equal(t, cty.String, ctyFunc.Params()[0].Type)
	require.Equal(t, cty.Number, ctyFunc.Params()[1].Type)
}

func TestCreateFunctionWithInvalidInParameterReturnsError(t *testing.T) {
	myfunc := func(a string, complex func() error) (int, error) {
		return 0, nil
	}

	_, err := createCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)
}

func TestCreateFunctionCreatesFunctionWithCorrectOutParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
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

	myfunc2 := func(a string, b int) int {
		return 1
	}

	_, err = createCtyFunctionFromGoFunc(myfunc2)
	require.Error(t, err)
}

func TestCreateFunctionCallsFunction(t *testing.T) {
	myfunc := func(a, b int) (int, error) {
		return a + b, nil
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	val, err := ctyFunc.Call([]cty.Value{cty.NumberIntVal(2), cty.NumberIntVal(3)})
	require.NoError(t, err)

	bf := val.AsBigFloat()
	i, _ := bf.Int64()
	require.Equal(t, int64(5), i)
}

func TestCreateFunctionHandlesInputParams(t *testing.T) {
	type testCase struct {
		name string
		f    interface{}
	}

	cases := []testCase{
		{
			name: "integer input parameters",
			f: func(a, b int) (int, error) {
				return a + b, nil
			},
		},
		{
			name: "int16 input parameters",
			f: func(a, b int16) (int16, error) {
				return a + b, nil
			},
		},
		{
			name: "int32 input parameters",
			f: func(a, b int32) (int32, error) {
				return a + b, nil
			},
		},
		{
			name: "int64 input parameters",
			f: func(a, b int64) (int64, error) {
				return a + b, nil
			},
		},
		{
			name: "uint input parameters",
			f: func(a, b uint) (uint, error) {
				return a + b, nil
			},
		},
		{
			name: "uint16 input parameters",
			f: func(a, b uint16) (uint16, error) {
				return a + b, nil
			},
		},
		{
			name: "uint32 input parameters",
			f: func(a, b uint32) (uint32, error) {
				return a + b, nil
			},
		},
		{
			name: "uint64 input parameters",
			f: func(a, b uint64) (uint64, error) {
				return a + b, nil
			},
		},
		{
			name: "float32 input parameters",
			f: func(a, b float32) (float32, error) {
				return a + b, nil
			},
		},
		{
			name: "float64 input parameters",
			f: func(a, b float64) (float64, error) {
				return a + b, nil
			},
		},
		{
			name: "string input parameters",
			f: func(a, b string) (string, error) {
				return a + b, nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := createCtyFunctionFromGoFunc(c.f)
			require.NoError(t, err)
		})
	}
}
