package hclconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/infinytum/raymond/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

func createCtyFunctionFromGoFunc(f any) (function.Function, error) {
	// get the parameters
	inParams := []function.Parameter{}
	var outParam function.TypeFunc

	var outType reflect.Kind
	inType := []reflect.Kind{}

	rf := reflect.TypeOf(f)

	if rf.NumOut() != 2 {
		return function.Function{}, fmt.Errorf("HCL functions must return two parameters, the result and an error i.e func(a,b int) (int, error)")
	}

	for i := 0; i < rf.NumIn(); i++ {
		fp := rf.In(i)

		switch fp.Kind() {
		case reflect.String:
			appendParms(&inType, &inParams, fp.Name(), reflect.String, cty.String)
		case reflect.Int16:
			appendParms(&inType, &inParams, fp.Name(), reflect.Int16, cty.Number)
		case reflect.Int32:
			appendParms(&inType, &inParams, fp.Name(), reflect.Int32, cty.Number)
		case reflect.Int64:
			appendParms(&inType, &inParams, fp.Name(), reflect.Int64, cty.Number)
		case reflect.Int:
			appendParms(&inType, &inParams, fp.Name(), reflect.Int, cty.Number)
		case reflect.Uint:
			appendParms(&inType, &inParams, fp.Name(), reflect.Uint, cty.Number)
		case reflect.Uint16:
			appendParms(&inType, &inParams, fp.Name(), reflect.Uint16, cty.Number)
		case reflect.Uint32:
			appendParms(&inType, &inParams, fp.Name(), reflect.Uint32, cty.Number)
		case reflect.Uint64:
			appendParms(&inType, &inParams, fp.Name(), reflect.Uint64, cty.Number)
		case reflect.Float32:
			appendParms(&inType, &inParams, fp.Name(), reflect.Float32, cty.Number)
		case reflect.Float64:
			appendParms(&inType, &inParams, fp.Name(), reflect.Float64, cty.Number)
		default:
			return function.Function{}, fmt.Errorf("type %v is not a valid cyt type, only primative types like string and basic numbers are supported", fp.Kind())
		}
	}

	outType = rf.Out(0).Kind()
	switch outType {
	case reflect.String:
		outParam = function.StaticReturnType(cty.String)
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		fallthrough
	case reflect.Int:
		fallthrough
	case reflect.Uint:
		outParam = function.StaticReturnType(cty.Number)
	case reflect.Bool:
		outParam = function.StaticReturnType(cty.Bool)
	default:
		return function.Function{}, fmt.Errorf("type %v is not a valid cyt type, only primative types like string and basic numbers are supported", rf.Out(0).Kind())
	}

	return function.New(&function.Spec{
		Params: inParams,
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {

			// create the params
			in := []reflect.Value{}
			for i, a := range args {
				switch a.Type() {
				case cty.String:
					in = append(in, reflect.ValueOf(a.AsString()))
				case cty.Number:
					bf := a.AsBigFloat()
					switch inType[i] {
					case reflect.Int16:
						val, _ := bf.Int64()
						in = append(in, reflect.ValueOf(int16(val)))
					case reflect.Int32:
						val, _ := bf.Int64()
						in = append(in, reflect.ValueOf(int32(val)))
					case reflect.Int64:
						val, _ := bf.Int64()
						in = append(in, reflect.ValueOf(int64(val)))
					case reflect.Int:
						val, _ := bf.Int64()
						in = append(in, reflect.ValueOf(int(val)))
					case reflect.Uint16:
						val, _ := bf.Uint64()
						in = append(in, reflect.ValueOf(uint16(val)))
					case reflect.Uint32:
						val, _ := bf.Uint64()
						in = append(in, reflect.ValueOf(uint32(val)))
					case reflect.Uint64:
						val, _ := bf.Uint64()
						in = append(in, reflect.ValueOf(uint64(val)))
					case reflect.Uint:
						val, _ := bf.Uint64()
						in = append(in, reflect.ValueOf(uint(val)))
					case reflect.Float32:
						val, _ := bf.Float64()
						in = append(in, reflect.ValueOf(float32(val)))
					case reflect.Float64:
						val, _ := bf.Float64()
						in = append(in, reflect.ValueOf(float64(val)))
					}
				case cty.Bool:
					in = append(in, reflect.ValueOf(a.True()))
				}
			}

			out := reflect.ValueOf(f).Call(in)

			switch retType {
			case cty.String:
				if out[1].Interface() == nil {
					return cty.StringVal(out[0].String()), nil
				} else {
					return cty.StringVal(out[0].String()), out[1].Interface().(error)
				}
			case cty.Number:
				switch outType {
				case reflect.Int16:
					fallthrough
				case reflect.Int32:
					fallthrough
				case reflect.Int64:
					fallthrough
				case reflect.Int:
					if out[1].Interface() == nil {
						return cty.NumberIntVal(out[0].Int()), nil
					} else {
						return cty.NumberIntVal(out[0].Int()), out[1].Interface().(error)
					}
				case reflect.Uint16:
					fallthrough
				case reflect.Uint32:
					fallthrough
				case reflect.Uint64:
					fallthrough
				case reflect.Uint:
					if out[1].Interface() == nil {
						return cty.NumberUIntVal(out[0].Uint()), nil
					} else {
						return cty.NumberUIntVal(out[0].Uint()), out[1].Interface().(error)
					}
				case reflect.Float32:
					fallthrough
				case reflect.Float64:
					if out[1].Interface() == nil {
						return cty.NumberFloatVal(out[0].Float()), nil
					} else {
						return cty.NumberFloatVal(out[0].Float()), out[1].Interface().(error)
					}
				}
			case cty.Bool:
				if out[1].Interface() == nil {
					return cty.BoolVal(out[0].Bool()), nil
				} else {
					return cty.BoolVal(out[0].Bool()), out[1].Interface().(error)
				}
			}

			return cty.NullVal(retType), nil
		},
		Type: outParam,
	}), nil
}

func appendParms(inType *[]reflect.Kind, params *[]function.Parameter, name string, kind reflect.Kind, typ cty.Type) {
	*inType = append(*inType, kind)
	*params = append(*params, function.Parameter{
		Name:             name,
		Type:             typ,
		AllowDynamicType: true,
	})
}

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

			tmpl.RegisterHelpers(map[string]any{
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
