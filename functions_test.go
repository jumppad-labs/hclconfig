package hclconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/hclconfig/test_fixtures/structs"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type TestStruct struct {
	Name        string      `hcl:"name" json:"name"`               // http_post
	Description string      `hcl:"description" json:"description"` // a http post is executed
	Type        string      `hcl:"type" json:"type"`               // command, parameter, comparitor
	Parameters  []cty.Value `hcl:"parameters" json:"parameters"`   // input params that function gets
}

func TestCreateCTYFunctionCreatesFunctionWithCorrectInParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	require.Equal(t, cty.String, ctyFunc.Params()[0].Type)
	require.Equal(t, cty.Number, ctyFunc.Params()[1].Type)
}

func TestCreateCTYFunctionWithInvalidInParameterReturnsError(t *testing.T) {
	myfunc := func(a string, complex func() error) (int, error) {
		return 0, nil
	}

	_, err := createCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)
}

func TestCreateCTYFunctionCreatesFunctionWithCorrectOutParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	rt, err := ctyFunc.ReturnType([]cty.Type{cty.String, cty.Number})
	require.NoError(t, err)
	require.Equal(t, cty.Number, rt)
}

func TestCreateCTYFunctionWithInvalidOutParameterReturnsError(t *testing.T) {
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

func TestCreateCTYFunctionCallsFunctionWithFloat(t *testing.T) {
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

func TestCreateCTYFunctionCallsFunctionWithMap(t *testing.T) {
	myfunc := func(a, b map[string]string) (map[string]string, error) {
		// add a to b
		for k, v := range a {
			b[k] = v
		}

		return b, nil
	}

	ctyFunc, err := createCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	a := cty.MapVal(map[string]cty.Value{
		"one": cty.StringVal("1"),
		"two": cty.StringVal("2"),
	})

	b := cty.MapVal(map[string]cty.Value{
		"three": cty.StringVal("3"),
		"four":  cty.StringVal("4"),
	})

	val, err := ctyFunc.Call([]cty.Value{a, b})
	require.NoError(t, err)

	bf := val.AsValueMap()
	require.Equal(t, cty.StringVal("1"), bf["one"])
}

func TestParseProcessesDefaultFunctionsWithFile(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/functions/default.hcl")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MYENV", "myvalue")
	t.Cleanup(func() {
		os.Unsetenv("MYENV")
	})

	home, _ := os.UserHomeDir()

	p := setupParser(t)
	c, err := p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.default")
	require.NoError(t, err)

	cont := r.(*structs.Container)

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
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/functions")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MYENV", "myvalue")
	t.Cleanup(func() {
		os.Unsetenv("MYENV")
	})

	home, _ := os.UserHomeDir()

	p := setupParser(t)
	p.RegisterFunction("constant_number", func() (int, error) { return 42, nil })

	c, err := p.ParseDirectory(absoluteFolderPath)
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.default")
	require.NoError(t, err)

	cont := r.(*structs.Container)

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
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/functions/custom.hcl")
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

func TestCreateTestFunctionReturnsErrorIfFunctionNotHas2ReturnParams(t *testing.T) {
	_, err := createCtyTestFunctionFromGoFunc("test", "this is a test func", TestFuncCommand, func() {})
	require.Error(t, err)
}

func TestCreateTestFunctionReturnsErrorIfFunctionNotHas2InputParams(t *testing.T) {
	_, err := createCtyTestFunctionFromGoFunc(
		"test",
		"this is a test func",
		TestFuncCommand,
		func() (context.Context, error) {
			return context.TODO(), nil
		},
	)
	require.Error(t, err)
}

func TestCreateTestFunctionCreatesFunctionWithCorrectParameters(t *testing.T) {
	f, err := createCtyTestFunctionFromGoFunc(
		"test",
		"this is a test func",
		TestFuncCommand,
		func(context.Context, TestLogger, string, int) (context.Context, error) {
			return context.TODO(), nil
		},
	)
	require.NoError(t, err)

	require.Equal(t, cty.String, f.Params()[0].Type)
	require.Equal(t, cty.Number, f.Params()[1].Type)
}

func TestExecuteTestFunctionReturnsCorrectOutput(t *testing.T) {
	f, err := createCtyTestFunctionFromGoFunc(
		"test",
		"this is a test func",
		TestFuncCommand,
		func(context.Context, TestLogger, string, int) (context.Context, error) {
			return context.TODO(), nil
		},
	)
	require.NoError(t, err)

	out, err := f.Call([]cty.Value{cty.StringVal("hello world"), cty.NumberIntVal(1)})
	require.NoError(t, err)

	require.True(t, out.Type().IsObjectType())
	require.Equal(t, cty.StringVal("test"), out.AsValueMap()["name"])
	require.Equal(t, cty.StringVal(TestFuncCommand), out.AsValueMap()["type"])
	require.Equal(t, cty.StringVal("this is a test func"), out.AsValueMap()["description"])

	// output parameters are encoded as a json array
	require.Equal(t, cty.StringVal(`["hello world",1]`), out.AsValueMap()["parameters"])
}
