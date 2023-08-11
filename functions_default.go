package hclconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mailgun/raymond/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

func getDefaultFunctions(filePath string) map[string]function.Function {
	var EnvFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "env",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(os.Getenv(args[0].AsString())), nil
		},
	})

	var HomeFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			h, _ := os.UserHomeDir()
			return cty.StringVal(h), nil
		},
	})

	var ReadFileFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "path",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// convert the file path to an absolute
			fp := ensureAbsolute(args[0].AsString(), filePath)

			// read the contents of the file
			d, err := os.ReadFile(fp)
			if err != nil {
				return cty.StringVal(""), err
			}

			return cty.StringVal(string(d)), nil
		},
	})

	var ReadTemplateFileFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "path",
				Type:             cty.String,
				AllowDynamicType: true,
			},
			{
				Name:             "variables",
				Type:             cty.DynamicPseudoType,
				AllowUnknown:     true,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// convert the file path to an absolute
			fp := ensureAbsolute(args[0].AsString(), filePath)

			// read the contents of the file
			d, err := os.ReadFile(fp)
			if err != nil {
				return cty.StringVal(""), err
			}

			vars := args[1]
			if vars.IsNull() || !vars.Type().IsObjectType() {
				return cty.StringVal(""), fmt.Errorf(`variables is either empty or not correctly formatted, e.g. { foo = "bar" list = ["a", "b"] number = 3 }`)
			}

			variables := ParseVars(vars.AsValueMap())

			tmpl, err := raymond.Parse(string(d))
			if err != nil {
				return cty.StringVal(""), fmt.Errorf("error parsing template: %s", err)
			}

			tmpl.RegisterHelpers(map[string]interface{}{
				"quote": func(in string) string {
					return fmt.Sprintf(`"%s"`, in)
				},
				"trim": func(in string) string {
					return strings.TrimSpace(in)
				},
			})

			result, err := tmpl.Exec(variables)
			if err != nil {
				return cty.StringVal(""), fmt.Errorf("error processing template: %s", err)
			}

			return cty.StringVal(result), nil
		},
	})

	var LenFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "var",
				Type:             cty.DynamicPseudoType,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.Number),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) == 1 && args[0].Type().IsCollectionType() || args[0].Type().IsTupleType() {
				i := args[0].ElementIterator()
				if i.Next() {
					return args[0].Length(), nil
				}
			}

			if len(args) == 1 && args[0].Type() == cty.String {
				return cty.NumberIntVal(int64(len(args[0].AsString()))), nil
			}

			return cty.NumberIntVal(0), nil
		},
	})

	var DirFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			s, err := filepath.Abs(filePath)

			// check if filepath is already a directory
			if stat, err := os.Stat(s); err == nil && stat.IsDir() {
				return cty.StringVal(s), err
			}

			return cty.StringVal(filepath.Dir(s)), err
		},
	})

	var TrimFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "string",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(strings.TrimSpace(args[0].AsString())), nil
		},
	})

	var ElementFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "value",
				Type:             cty.DynamicPseudoType,
				AllowDynamicType: true,
			},
			{
				Name:             "index",
				Type:             cty.DynamicPseudoType,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if args[0].Type().IsTupleType() || args[0].Type().IsListType() {
				i := args[0].ElementIterator()

				for {
					if !i.Next() {
						break
					}

					index, e := i.Element()
					if index.Equals(args[1]).True() {
						return e, nil
					}
				}

				return cty.NullVal(retType), nil
			} else if args[1].Type() == cty.String && (args[0].Type().IsObjectType() || args[0].Type().IsMapType()) {
				index := args[1].AsString()
				m := args[0].AsValueMap()

				return m[index], nil
			}

			return cty.NullVal(retType), nil
		},
	})

	//var ToString = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "value",
	//			Type:             cty.DynamicPseudoType,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.DynamicPseudoType),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		if args[0].Type().IsTupleType() || args[0].Type().IsListType() {
	//			vals := []cty.Value{}
	//			i := args[0].ElementIterator()

	//			for {
	//				if !i.Next() {
	//					break
	//				}

	//				, e := i.Element()
	//			}

	//		return cty.NullVal(retType), nil
	//	},
	//})

	funcs := map[string]function.Function{
		"abs":             stdlib.AbsoluteFunc,
		"ceil":            stdlib.CeilFunc,
		"chomp":           stdlib.ChompFunc,
		"coalescelist":    stdlib.CoalesceListFunc,
		"compact":         stdlib.CompactFunc,
		"concat":          stdlib.ConcatFunc,
		"contains":        stdlib.ContainsFunc,
		"csvdecode":       stdlib.CSVDecodeFunc,
		"dir":             DirFunc,
		"distinct":        stdlib.DistinctFunc,
		"element":         ElementFunc,
		"env":             EnvFunc,
		"chunklist":       stdlib.ChunklistFunc,
		"file":            ReadFileFunc,
		"flatten":         stdlib.FlattenFunc,
		"floor":           stdlib.FloorFunc,
		"format":          stdlib.FormatFunc,
		"formatdate":      stdlib.FormatDateFunc,
		"formatlist":      stdlib.FormatListFunc,
		"home":            HomeFunc,
		"indent":          stdlib.IndentFunc,
		"join":            stdlib.JoinFunc,
		"jsondecode":      stdlib.JSONDecodeFunc,
		"jsonencode":      stdlib.JSONEncodeFunc,
		"keys":            stdlib.KeysFunc,
		"len":             LenFunc,
		"log":             stdlib.LogFunc,
		"lower":           stdlib.LowerFunc,
		"max":             stdlib.MaxFunc,
		"merge":           stdlib.MergeFunc,
		"min":             stdlib.MinFunc,
		"parseint":        stdlib.ParseIntFunc,
		"pow":             stdlib.PowFunc,
		"range":           stdlib.RangeFunc,
		"regex":           stdlib.RegexFunc,
		"regexall":        stdlib.RegexAllFunc,
		"reverse":         stdlib.ReverseListFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setsubtract":     stdlib.SetSubtractFunc,
		"setunion":        stdlib.SetUnionFunc,
		"signum":          stdlib.SignumFunc,
		"slice":           stdlib.SliceFunc,
		"sort":            stdlib.SortFunc,
		"split":           stdlib.SplitFunc,
		"strrev":          stdlib.ReverseFunc,
		"substr":          stdlib.SubstrFunc,
		"template_file":   ReadTemplateFileFunc,
		"timeadd":         stdlib.TimeAddFunc,
		"title":           stdlib.TitleFunc,
		"trim":            TrimFunc,
		"trimprefix":      stdlib.TrimPrefixFunc,
		"trimspace":       stdlib.TrimSpaceFunc,
		"trimsuffix":      stdlib.TrimSuffixFunc,
		"upper":           stdlib.UpperFunc,
		"values":          stdlib.ValuesFunc,
		"zipmap":          stdlib.ZipmapFunc,
	}

	return funcs
}
