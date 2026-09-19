package ctydebug_test

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/cty/ctydebug"
	"github.com/jumppad-labs/xcl/internal/cty"
)

func ExampleDiffValues() {
	a := cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("a"),
		"bar": cty.StringVal("b"),
		"baz": cty.MapVal(map[string]cty.Value{
			"hello": cty.StringVal("world"),
		}),
	})
	b := cty.ObjectVal(map[string]cty.Value{
		"bar": cty.EmptyObjectVal,
		"baz": cty.MapVal(map[string]cty.Value{
			"hello":   cty.UnknownVal(cty.String),
			"goodbye": cty.StringVal("moon"),
		}),
	})

	fmt.Print(ctydebug.DiffValues(b, a))

	// Output:
	// {cty.Value}["bar"]
	//   got:  cty.StringVal("b")
	//   want: ctydebug.ctyObjectVal{}
	//
	// {cty.Value}["baz"]["goodbye"]
	//   got:  (no value)
	//   want: cty.StringVal("moon")
	//
	// {cty.Value}["baz"]["hello"]
	//   got:  cty.StringVal("world")
	//   want: cty.UnknownVal(cty.String)
	//
	// {cty.Value}["foo"]
	//   got:  cty.StringVal("a")
	//   want: (no value)

}
