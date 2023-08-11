package test

import (
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var TestFunction = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "var",
			Type:             cty.EmptyObject,
			AllowDynamicType: true,
			AllowNull:        true,
		},
	},
	Type: function.StaticReturnType(cty.DynamicPseudoType),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {

		// define the return function
		rf := map[string]cty.Value{
			"name": cty.StringVal("hello"),
			"type": cty.StringVal("command"),
			"parameters": cty.MapVal(map[string]cty.Value{
				"a": cty.StringVal("b"),
			}),
		}

		return cty.ObjectVal(rf), nil
	},
})
