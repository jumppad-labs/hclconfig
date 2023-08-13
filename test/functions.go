package test

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

// TestFunc is a function is a function that is executed by the testing process
// it has a manadator parameter of context that is used to recieive information regarding
// the current testing environment. If the current test run times out, then the context will be cancelled.
// params are optional parameters the the function will be called with, these will be the go types that
// are converted from the hcl function call parameters.
// If it needs to update the environment it does so by writing to context and returning.
type TestFunc func(ctx context.Context, l *Logger, params ...any) (context.Context, error)

// creteCtyTestFunctionFromGoFunc creates a CTY function using t
func CreateCtyTestFunctionFromGoFunc(name, description, typ string, f interface{}) (function.Function, error) {
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
		var err error
		inTypes, inParams, err = getInputParameters(rf)
		if err != nil {
			return function.Function{}, err
		}
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

	// skip the first parameter as that is always context
	for i := 2; i < rf.NumIn(); i++ {
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
