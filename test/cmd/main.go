package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/example/types"
	"github.com/jumppad-labs/hclconfig/test"
	htypes "github.com/jumppad-labs/hclconfig/types"
	"github.com/kylelemons/godebug/pretty"
	"golang.org/x/net/context"
)

// list of registered functions that the test process can call
var functions map[string]interface{}

func main() {
	o := hclconfig.DefaultOptions()

	// set the callback that will be executed when a resource has been created
	// this function can be used to execute any external work required for the
	// resource.
	o.ParseCallback = parseCallback
	p := hclconfig.NewParser(o)
	// register the types
	p.RegisterType("config", &types.Config{})
	p.RegisterType("postgres", &types.PostgreSQL{})
	p.RegisterType("scenario", &test.Scenario{})
	p.RegisterType("test", &test.Test{})

	// register a custom function
	p.RegisterFunction("random_number", func() (int, error) {
		return rand.Intn(100), nil
	})

	functions = map[string]interface{}{}

	// generate a test func
	f, err := test.CreateCtyTestFunctionFromGoFunc("http_post_func", "make a call to an HTTP server with the given url", "command", http_post_func)
	if err != nil {
		panic(err)
	}
	p.RegisterCTYFunction("http_post", f)
	functions["http_post_func"] = http_post_func

	f, err = test.CreateCtyTestFunctionFromGoFunc("with_headers", "with the headers", "parameter", with_headers)
	if err != nil {
		panic(err)
	}
	p.RegisterCTYFunction("with_headers", f)
	functions["with_headers"] = with_headers

	f, err = test.CreateCtyTestFunctionFromGoFunc("with_body", "with the body", "parameter", with_body)
	if err != nil {
		panic(err)
	}

	p.RegisterCTYFunction("with_body", f)
	functions["with_body"] = with_body

	f, err = test.CreateCtyTestFunctionFromGoFunc("body_contains", "the body contains", "assertion", body_contains)
	if err != nil {
		panic(err)
	}

	p.RegisterCTYFunction("body_contains", f)
	functions["body_contains"] = body_contains

	f, err = test.CreateCtyTestFunctionFromGoFunc("body", "output the body", "output", body)
	if err != nil {
		panic(err)
	}

	p.RegisterCTYFunction("body", f)
	functions["body"] = body

	f, err = test.CreateCtyTestFunctionFromGoFunc("return_status_code", "returns the status code", "assertion", return_status_code)
	if err != nil {
		panic(err)
	}

	p.RegisterCTYFunction("return_status_code", f)
	functions["return_status_code"] = return_status_code

	f, err = test.CreateCtyTestFunctionFromGoFunc("resources_are_created", "check resources are created", "command", resources_are_created)
	if err != nil {
		panic(err)
	}

	p.RegisterCTYFunction("resources_are_created", f)
	functions["resources_are_created"] = resources_are_created

	f, err = test.CreateCtyTestFunctionFromGoFunc("script", "execute the script", "command", script)
	if err != nil {
		panic(err)
	}

	p.RegisterCTYFunction("script", f)
	functions["script"] = script

	f, err = test.CreateCtyTestFunctionFromGoFunc("with_arguments", "with the arguments", "arguments", with_arguments)
	if err != nil {
		panic(err)
	}
	p.RegisterCTYFunction("with_arguments", f)
	functions["with_arguments"] = with_arguments

	f, err = test.CreateCtyTestFunctionFromGoFunc("have_an_exit_code", "should have the exit code", "assertion", have_an_exit_code)
	if err != nil {
		panic(err)
	}
	p.RegisterCTYFunction("have_an_exit_code", f)
	functions["have_an_exit_code"] = have_an_exit_code

	f, err = test.CreateCtyTestFunctionFromGoFunc("output", "and output", "assertion", output)
	if err != nil {
		panic(err)
	}
	p.RegisterCTYFunction("output", f)
	functions["output"] = output

	f, err = test.CreateCtyTestFunctionFromGoFunc("and", "and equal", "comparitor", and)
	if err != nil {
		panic(err)
	}
	p.RegisterCTYFunction("and", f)
	functions["and"] = and

	c, err := p.ParseDirectory("./example/scenarios")
	if err != nil {
		l := test.NewLogger(test.DefaultOptions())
		l.LogError(err.Error())
		os.Exit(1)
	}

	s, err := c.FindResource("resource.scenario.testing_modules")
	if err != nil {
		panic(err)
	}

	fmt.Println(s.Metadata().Name)
}

func parseCallback(r htypes.Resource) error {
	l := test.NewLogger(test.DefaultOptions())

	if r.Metadata().Type == "scenario" {
		s := r.(*test.Scenario)

		l.LogScenario(s.Description)

		// iterate each test and it and build a command
		for _, t := range s.Tests {
			for _, i := range t.Its {
				t := setupTestContext(l)

				l.LogIt(i.Description)

				// process this expectations, this is where we do any work
				err := processExpect(t, i.Expect)
				if err != nil {
					return fmt.Errorf("test failed, '%s' with error '%s'", i.Description, err)
				}

				// process the assertions
				err = processAssert(t, i.To)
				if err != nil {
					return fmt.Errorf("test failed, '%s' with error '%s'", i.Description, err)
				}
			}
		}
	}

	return nil
}

// to call a command we itterate backwards over a list of functions
// extracting the parameters which we pass to the
func processExpect(t *TestContext, funcs []test.FunctionDetails) error {
	ctx := t.Context

	// we need to execute expects backwards as the parameters will always come before the command
	for i := len(funcs) - 1; i >= 0; i-- {
		f := funcs[i]

		var err error
		ctx, err = t.CallFunction(f.Name, f.Description, f.Parameters, ctx)
		if err != nil {
			return fmt.Errorf("expectation error: %s", err)
		}
	}

	return nil
}

func processAssert(t *TestContext, funcs []test.FunctionDetails) error {
	ctx := t.Context
	for i := len(funcs) - 1; i >= 0; i-- {
		f := funcs[i]

		var err error
		ctx, err = t.CallFunction(f.Name, f.Description, f.Parameters, ctx)
		if err != nil {
			return fmt.Errorf("assertion error: %s", err)
		}
	}

	return nil
}

func setupTestContext(l *test.Logger) *TestContext {
	t := &TestContext{
		Functions: map[string]interface{}{},
		Context:   context.Background(),
		Logger:    l,
	}

	t.Functions = functions

	return t
}

type TestContext struct {
	Functions map[string]interface{} // functions are held as a reference
	Context   context.Context
	Logger    *test.Logger
}

func (t *TestContext) CallFunction(name string, description, p string, ctx context.Context) (context.Context, error) {
	f, ok := t.Functions[name]
	if !ok {
		return ctx, fmt.Errorf("function '%s' is not registered", name)
	}

	// we need to call the function using reflection
	rf := reflect.ValueOf(f)

	// we always pass the context and logger
	ctxVal := reflect.ValueOf(ctx)
	logVal := reflect.ValueOf(t.Logger)
	inParams := []reflect.Value{ctxVal, logVal}

	params := []json.RawMessage{}

	// first deserialize the parameters into an array
	json.Unmarshal([]byte(p), &params)

	// then do a second pass deserialzing the json into the correct type
	for i, p := range params {
		inPar := rf.Type().In(i + 2)
		inType := reflect.New(inPar)

		json.Unmarshal(p, inType.Interface())

		inParams = append(inParams, inType.Elem())
	}

	t.Logger.LogFunctionDescription(description)
	for i, p := range inParams {
		if i < 2 {
			continue
		}

		t.Logger.LogFunctionLine(pretty.Sprint(p.Interface()))
	}

	// then try to call the function using reflection
	out := rf.Call(inParams)

	// now fetch the context and the error from the output
	var e error
	c := out[0].Interface().(context.Context)

	if !out[1].IsNil() {
		e = out[1].Interface().(error)
	}

	return c, e
}
