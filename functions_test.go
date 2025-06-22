package hclconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/hclconfig/internal/test_fixtures/plugin/structs"
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
		f    any
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

func TestParseProcessesDefaultFunctionsWithFile(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./internal/test_fixtures/functions/default.hcl")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MYENV", "myvalue")
	t.Cleanup(func() {
		os.Unsetenv("MYENV")
	})

	p := setupParser(t)
	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.default")
	require.NoError(t, err)

	cont := r.(*structs.Container)

	home, _ := os.UserHomeDir()

	require.Equal(t, "3", cont.Env["len_string"])
	require.Equal(t, "2", cont.Env["len_collection"])
	require.Equal(t, "myvalue", cont.Env["env"])
	require.Equal(t, home, cont.Env["home"])
	require.Contains(t, cont.Env["file"], "container")
	require.Contains(t, cont.Env["dir"], filepath.Dir(absoluteFolderPath))
	require.Contains(t, cont.Env["trim"], "foo bar")
	require.Equal(t, "one", cont.DNS[0])
	require.Equal(t, "two", cont.DNS[1])
	require.Equal(t, "123", cont.Entrypoint[0])
	require.Equal(t, "abc", cont.Entrypoint[1])
	require.Equal(t, "one", cont.Command[0])
	require.Equal(t, "two", cont.Command[1])
}

func TestParseProcessesDefaultFunctionsWithDirectory(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./internal/test_fixtures/functions")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MYENV", "myvalue")
	t.Cleanup(func() {
		os.Unsetenv("MYENV")
	})

	p := setupParser(t)
	p.RegisterFunction("constant_number", func() (int, error) { return 42, nil })

	c, err := p.ParseDirectory(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.default")
	require.NoError(t, err)

	cont := r.(*structs.Container)

	home, _ := os.UserHomeDir()

	require.Equal(t, "3", cont.Env["len_string"])
	require.Equal(t, "2", cont.Env["len_collection"])
	require.Equal(t, "myvalue", cont.Env["env"])
	require.Equal(t, home, cont.Env["home"])
	require.Contains(t, cont.Env["file"], "container")
	require.Contains(t, cont.Env["dir"], absoluteFolderPath)
	require.Contains(t, cont.Env["trim"], "foo bar")

	// template
	require.Contains(t, cont.Env["template_file"], "Hello Raymond")
	require.Contains(t, cont.Env["template_file"], "43 is a number")
	require.Contains(t, cont.Env["template_file"], "cheese\n  ham\n  pineapple")
	require.Contains(t, cont.Env["template_file"], "foo = bar")
	require.Contains(t, cont.Env["template_file"], "x = 1")
}

func TestParseProcessesCustomFunctions(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./internal/test_fixtures/functions/custom.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)
	p.RegisterFunction("constant_number", func() (int, error) { return 42, nil })

	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.custom")
	require.NoError(t, err)

	cont := r.(*structs.Container)

	require.Equal(t, "42", cont.Env["len"])
}
