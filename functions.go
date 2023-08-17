package hclconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/jumppad-labs/hclconfig/convert"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
)

// HCLFunc defines a signature for custom HCL functions
// Functions are standard go functions that have 0 or more parameters, and
// return two parameters of a type and an error
type HCLFunc func(parmeters ...any) (interface{}, error)

// TestLogger is a logger that is injected into test functions that allows basic
// output
type TestLogger interface {
	Log(message string, parameters ...interface{})
}

// TestFunc is a function is a function that is executed by the testing process
// it has a manadator parameter of context that is used to recieive information regarding
// the current testing environment. If the current test run times out, then the context will be cancelled.
// params are optional parameters the the function will be called with, these will be the go types that
// are converted from the hcl function call parameters.
// If it needs to update the environment it does so by writing to context and returning.
type TestFunc func(ctx context.Context, l *TestLogger, params ...any) (context.Context, error)

const (
	// TestFuncCommand defines the type of a command test function that
	// performs an action
	TestFuncCommand string = "command"
	// TestFuncParameter defines the type of a parameter test func that adds
	// parameters to commands
	TestFuncParameter string = "parameter"
	// TestFuncAssertion defines the type of an assertion function that is
	// asserts the value of something
	TestFuncAssertion string = "assert"
	// TestFuncOperand defines the type of a operand function that
	// combines one or more assertions, e.g. and, or, less_than
	TestFuncOperand string = "comparitor"
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

// creteCtyTestFunctionFromGoFunc creates a CTY function using t
func createCtyTestFunctionFromGoFunc(name, description, typ string, f interface{}) (function.Function, error) {
	rf := reflect.TypeOf(f)

	// validate the output parameters
	if rf.NumOut() != 2 {
		return function.Function{}, fmt.Errorf("error with function %s, test functions must return two parameters, context.Context and an error i.e func(ctx context.Context, a,b int) (context.Context, error)", rf.String())
	}

	// validate the input parameters
	if rf.NumIn() < 2 {
		return function.Function{}, fmt.Errorf("error with function %s, test functions must accept a minimum of two parameter, context.Context i.e func(ctx context.Context, a,b int) (context.Context, error)", rf.String())
	}

	inParams := []function.Parameter{}
	inTypes := []reflect.Type{}
	if rf.NumIn() > 2 {
		// inputParams will always error as context.Context can not be converted
		inTypes, inParams, _ = getInputParameters(rf)

		// trim the first two as these are default params
		inTypes = inTypes[2:]
		inParams = inParams[2:]
	}

	return function.New(&function.Spec{
		// remove the context parameter
		Params: inParams,
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {

			// return the output
			output := map[string]cty.Value{}
			output["name"] = cty.StringVal(name)
			output["type"] = cty.StringVal(typ)
			output["description"] = cty.StringVal(description)
			output["parameters"] = cty.StringVal("")

			if len(args) > 0 {
				// convert the cty parameters to go types and serialize to json
				outParams := []interface{}{}

				for i, a := range args {
					// create a new input value based on the type
					val := reflect.New(inTypes[i])
					inst := val.Interface()

					// convert the cty parameter to a go type
					err := convert.CtyToGo(a, inst)
					if err != nil {
						return cty.NullVal(retType), fmt.Errorf("unable to convert input parameter: %s", err)
					}

					outParams = append(outParams, inst)
				}

				// convert to json
				d, _ := json.Marshal(outParams)
				output["parameters"] = cty.StringVal(string(d))
			}

			return cty.ObjectVal(output), nil
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
	}), nil
}

func getInputParameters(rf reflect.Type) ([]reflect.Type, []function.Parameter, error) {
	inTypes := []reflect.Type{}
	inParams := []function.Parameter{}

	var returnErr error

	for i := 0; i < rf.NumIn(); i++ {
		funcParam := rf.In(i)

		// create an instance of our type
		inVal := reflect.New(funcParam).Interface()

		// pass that to ImpliedType to get the cty Type
		ctyType, err := gocty.ImpliedType(inVal)

		// if we can not convert the type, create a nil type
		if err != nil {
			if returnErr != nil {
				returnErr = fmt.Errorf("%s, %s", returnErr, err)
			} else {
				returnErr = err
			}

			inTypes = append(inTypes, funcParam)
			inParams = append(inParams, function.Parameter{
				Name: funcParam.Name(),
				Type: cty.NilType,
			})

			continue
		}

		// set the go type
		inTypes = append(inTypes, funcParam)

		inParams = append(inParams, function.Parameter{
			Name:             funcParam.Name(),
			Type:             ctyType,
			AllowDynamicType: true,
		})
	}

	return inTypes, inParams, returnErr
}

func getOutputParameter(t reflect.Type) (function.TypeFunc, error) {
	val := reflect.New(t).Interface()
	ctyType, err := gocty.ImpliedType(val)
	if err != nil {
		return nil, err
	}

	return function.StaticReturnType(ctyType), nil
}
