package functions

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/jumppad-labs/xcl/internal/cty"
)

func TestCreateFunctionCreatesFunctionWithCorrectInParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	require.Equal(t, cty.String, ctyFunc.Params()[0].Type)
	require.Equal(t, cty.Number, ctyFunc.Params()[1].Type)
}

func TestCreateFunctionWithInvalidInParameterReturnsError(t *testing.T) {
	myfunc := func(a string, complex func() error) (int, error) {
		return 0, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)
}

func TestCreateFunctionCreatesFunctionWithCorrectOutParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := CreateCtyFunctionFromGoFunc(myfunc)
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

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)

	myfunc2 := func(a string, b int) int {
		return 1
	}

	_, err = CreateCtyFunctionFromGoFunc(myfunc2)
	require.Error(t, err)
}

func TestCreateFunctionCallsFunction(t *testing.T) {
	myfunc := func(a, b int) (int, error) {
		return a + b, nil
	}

	ctyFunc, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	val, err := ctyFunc.Call([]cty.Value{cty.NumberIntVal(2), cty.NumberIntVal(3)})
	require.NoError(t, err)

	bf := val.AsBigFloat()
	i, _ := bf.Int64()
	require.Equal(t, int64(5), i)
}

func TestCreateFunctionHandlesIntegerInputParams(t *testing.T) {
	myfunc := func(a, b int) (int, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesInt16InputParams(t *testing.T) {
	myfunc := func(a, b int16) (int16, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesInt32InputParams(t *testing.T) {
	myfunc := func(a, b int32) (int32, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesInt64InputParams(t *testing.T) {
	myfunc := func(a, b int64) (int64, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUintInputParams(t *testing.T) {
	myfunc := func(a, b uint) (uint, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUint16InputParams(t *testing.T) {
	myfunc := func(a, b uint16) (uint16, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUint32InputParams(t *testing.T) {
	myfunc := func(a, b uint32) (uint32, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUint64InputParams(t *testing.T) {
	myfunc := func(a, b uint64) (uint64, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesFloat32InputParams(t *testing.T) {
	myfunc := func(a, b float32) (float32, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesFloat64InputParams(t *testing.T) {
	myfunc := func(a, b float64) (float64, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesStringInputParams(t *testing.T) {
	myfunc := func(a, b string) (string, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}
