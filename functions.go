package hclconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func createCtyFunctionFromGoFunc(f interface{}) (function.Function, error) {
	// get the parameters
	params := []function.Parameter{}

	rf := reflect.TypeOf(f)
	for i := 0; i < rf.NumIn(); i++ {
		fp := rf.In(i)

		switch fp.Kind() {
		case reflect.String:
			params = append(params, function.Parameter{
				Name:             fp.Name(),
				Type:             cty.String,
				AllowDynamicType: true,
			})
		case reflect.Int16:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
			fallthrough
		case reflect.Int:
			params = append(params, function.Parameter{
				Name:             fp.Name(),
				Type:             cty.Number,
				AllowDynamicType: true,
			})
		default:
			return function.Function{}, fmt.Errorf("type %v is not a valid cyt type, only primative types like string and basic numbers are supported", fp.Kind())
		}
	}

	var outType function.TypeFunc
	outParam := rf.Out(0)
	switch outParam.Kind() {
	case reflect.String:
		outType = function.StaticReturnType(cty.String)
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Int:
		outType = function.StaticReturnType(cty.Number)
	default:
		return function.Function{}, fmt.Errorf("type %v is not a valid cyt type, only primative types like string and basic numbers are supported", rf.Out(0).Kind())
	}

	return function.New(&function.Spec{
		Params: params,
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {

			// create the params
			in := []reflect.Value{}
			for _, a := range args {
				switch a.Type() {
				case cty.String:
					in = append(in, reflect.ValueOf(a.AsString()))
				case cty.Number:
					bf := a.AsBigFloat()
					val, _ := bf.Int64()
					in = append(in, reflect.ValueOf(int(val)))
				}
			}

			out := reflect.ValueOf(f).Call(in)

			switch retType {
			case cty.Number:
				return cty.NumberIntVal(out[0].Int()), nil

			}

			return cty.NullVal(retType), nil
		},
		Type: outType,
	}), nil
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

	var FileFunc = function.New(&function.Spec{
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
			d, err := ioutil.ReadFile(fp)
			if err != nil {
				return cty.StringVal(""), err
			}

			return cty.StringVal(string(d)), nil
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

			return cty.NumberIntVal(0), nil
		},
	})

	var DirFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			s, err := filepath.Abs(filePath)

			return cty.StringVal(filepath.Dir(s)), err
		},
	})

	funcs := map[string]function.Function{}

	funcs["len"] = LenFunc
	funcs["env"] = EnvFunc
	funcs["home"] = HomeFunc
	funcs["file"] = FileFunc
	funcs["dir"] = DirFunc

	return funcs
}
