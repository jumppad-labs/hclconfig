package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/functions"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty/function"
)

// constantNumberOptions returns parser options with a custom function
// constant_number that always returns 42
func constantNumberOptions(t *testing.T) *ParserOptions {
	constantNumber, err := functions.CreateCtyFunctionFromGoFunc(func() (int, error) { return 42, nil })
	require.NoError(t, err)

	o := DefaultOptions()
	o.CustomFunctions = map[string]function.Function{
		"constant_number": constantNumber,
	}

	return o
}

func TestApplyProcessesDefaultFunctionsWithFile(t *testing.T) {
	absoluteFilePath, err := filepath.Abs("../test_fixtures/functions/default.xcl")
	require.NoError(t, err)

	t.Setenv("MYENV", "myvalue")

	p, _ := setupParser(t)
	c, err := p.Apply(absoluteFilePath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.default")

	home, _ := os.UserHomeDir()

	require.Equal(t, "3", cont.Env["len_string"])
	require.Equal(t, "2", cont.Env["len_collection"])
	require.Equal(t, "myvalue", cont.Env["env"])
	require.Equal(t, home, cont.Env["home"])
	require.Contains(t, cont.Env["file"], "container")
	require.Equal(t, filepath.Dir(absoluteFilePath), cont.Env["dir"])
	require.Equal(t, "foo bar", cont.Env["trim"])
	require.Equal(t, "one", cont.DNS[0])
	require.Equal(t, "two", cont.DNS[1])
	require.Equal(t, "123", cont.Entrypoint[0])
	require.Equal(t, "abc", cont.Entrypoint[1])
	require.Equal(t, "one", cont.Command[0])
	require.Equal(t, "two", cont.Command[1])
}

func TestApplyProcessesDefaultFunctionsWithDirectory(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("../test_fixtures/functions")
	require.NoError(t, err)

	t.Setenv("MYENV", "myvalue")

	p, _ := setupParser(t, constantNumberOptions(t))
	c, err := p.Apply(absoluteFolderPath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.default")

	home, _ := os.UserHomeDir()

	require.Equal(t, "3", cont.Env["len_string"])
	require.Equal(t, "2", cont.Env["len_collection"])
	require.Equal(t, "myvalue", cont.Env["env"])
	require.Equal(t, home, cont.Env["home"])
	require.Contains(t, cont.Env["file"], "container")
	require.Equal(t, absoluteFolderPath, cont.Env["dir"])
	require.Equal(t, "foo bar", cont.Env["trim"])

	// template
	require.Contains(t, cont.Env["template_file"], "Hello Raymond")
	require.Contains(t, cont.Env["template_file"], "43 is a number")
	require.Contains(t, cont.Env["template_file"], "cheese\n  ham\n  pineapple")
	require.Contains(t, cont.Env["template_file"], "foo = bar")
	require.Contains(t, cont.Env["template_file"], "x = 1")
}

func TestApplyProcessesCustomFunctions(t *testing.T) {
	absoluteFilePath, err := filepath.Abs("../test_fixtures/functions/custom.xcl")
	require.NoError(t, err)

	p, _ := setupParser(t, constantNumberOptions(t))
	c, err := p.Apply(absoluteFilePath)
	require.NoError(t, err)

	cont := findResource[structs.Container](t, c, "resource.container.custom")

	require.Equal(t, "42", cont.Env["len"])
}
