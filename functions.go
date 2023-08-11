package hclconfig

import (
	"fmt"
	"reflect"

	"github.com/jumppad-labs/hclconfig/convert"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
)

func createCtyFunctionFromGoFunc(f interface{}) (function.Function, error) {
	// get the parameters

	//var outType reflect.Kind

	rf := reflect.TypeOf(f)

	if rf.NumOut() != 2 {
		return function.Function{}, fmt.Errorf("HCL functions must return two parameters, the result and an error i.e func(a,b int) (int, error)")
	}

	inTypes, inParams, err := getInputParameters(rf)
	if err != nil {
		return function.Function{}, err
	}

	outParam, err := getOutputParameter(rf.Out(0))
	if err != nil {
		return function.Function{}, err
	}

	return function.New(&function.Spec{
		Params: inParams,
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {

			// build a collection of input parameters converted from the cty parameters
			in := []reflect.Value{}

			for i, a := range args {
				// create a new input value based on the type
				val := reflect.New(inTypes[i])
				inst := val.Interface()

				// convert the cty parameter to a go type
				err := convert.CtyToGo(a, inst)
				if err != nil {
					return cty.NullVal(retType), fmt.Errorf("unable to convert input parameter: %s", err)
				}

				in = append(in, val.Elem())
			}

			// call the function
			out := reflect.ValueOf(f).Call(in)

			// convert the output parameter to a cty type
			val, err := convert.GoToCtyValue(out[0].Interface())
			if err != nil {
				return cty.NullVal(retType), fmt.Errorf("unable to convert output parameter: %s", err)
			}

			var retErr error
			if !out[1].IsNil() {
				retErr = out[1].Interface().(error)
			}

			// return the output
			return val, retErr
		},
		Type: outParam,
	}), nil
}

func getInputParameters(rf reflect.Type) ([]reflect.Type, []function.Parameter, error) {
	inTypes := []reflect.Type{}
	inParams := []function.Parameter{}

	for i := 0; i < rf.NumIn(); i++ {
		funcParam := rf.In(i)

		// create an instance of our type
		inVal := reflect.New(funcParam).Interface()

		// pass that to ImpliedType to get the cty Type
		ctyType, err := gocty.ImpliedType(inVal)
		if err != nil {
			return nil, nil, err
		}

		// set the go type
		inTypes = append(inTypes, funcParam)

		inParams = append(inParams, function.Parameter{
			Name:             funcParam.Name(),
			Type:             ctyType,
			AllowDynamicType: true,
		})
	}

	return inTypes, inParams, nil
}

func getOutputParameter(t reflect.Type) (function.TypeFunc, error) {
	val := reflect.New(t).Interface()
	ctyType, err := gocty.ImpliedType(val)
	if err != nil {
		return nil, err
	}

	return function.StaticReturnType(ctyType), nil
}
