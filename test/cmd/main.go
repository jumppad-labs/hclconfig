package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/example/types"
	"github.com/jumppad-labs/hclconfig/test"
	htypes "github.com/jumppad-labs/hclconfig/types"
	"golang.org/x/net/context"
)

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

	p.RegisterFunction("resources_are_created", func(res []string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "resources_are_created",
		}, nil
	})

	p.RegisterFunction("http_post", func(uri string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name:       "http_post",
			Parameters: paramsToString(uri),
		}, nil
	})

	p.RegisterFunction("script", func(path string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "script",
		}, nil
	})

	p.RegisterFunction("with_headers", func(headers map[string]string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "with_headers",
		}, nil
	})

	p.RegisterFunction("with_arguments", func(headers map[string]string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "with_arguments",
		}, nil
	})

	p.RegisterFunction("with_body", func(body string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "with_body",
		}, nil
	})

	p.RegisterFunction("return_status_code", func(code int) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "return_status_code",
		}, nil
	})

	p.RegisterFunction("have_an_exit_code", func(code int) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "has_an_exit_code",
		}, nil
	})

	p.RegisterFunction("and", func() (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "and",
		}, nil
	})

	p.RegisterFunction("body_contains", func(contents string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "body_contains",
		}, nil
	})

	p.RegisterFunction("body", func() (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "body_contains",
		}, nil
	})

	p.RegisterFunction("output", func(out string) (test.FunctionDetails, error) {
		return test.FunctionDetails{
			Name: "body_contains",
		}, nil
	})

	fmt.Println("## Parse the config")
	c, err := p.ParseDirectory("./example/scenarios")
	if err != nil {
		fmt.Printf("An error occurred processing the config: %s\n", err)
		os.Exit(1)
	}

	s, err := c.FindResource("resource.scenario.testing_modules")
	if err != nil {
		panic(err)
	}

	fmt.Println(s.Metadata().Name)
}

func parseCallback(r htypes.Resource) error {
	fmt.Printf("resource '%s' named '%s' has been parsed from the file: %s\n", r.Metadata().Type, r.Metadata().Name, r.Metadata().File)

	if r.Metadata().Type == "scenario" {
		fmt.Println("")
		// iterate each test and it and build a command
		for _, t := range r.(*test.Scenario).Tests {
			for _, i := range t.Its {
				fmt.Println(i.Description)
				processCommand(i.Expect)
			}
		}
	}

	return nil
}

type TestContext struct {
	Functions map[string]func(params string, ctx context.Context) (context.Context, error)
	Context   context.Context
}

func (t *TestContext) CallFunction(name string, p string, ctx context.Context) (context.Context, error) {
	f, ok := t.Functions[name]
	if !ok {
		return ctx, fmt.Errorf("function '%s' is not registered", name)
	}

	return f(p, ctx)
}

func setupTestContext() *TestContext {
	t := &TestContext{
		Functions: map[string]func(params string, c context.Context) (context.Context, error){},
		Context:   context.Background(),
	}

	t.Functions["http_post"] = http_post_func

	return t
}

// to call a command we itterate backwards over a list of functions
// extracting the parameters which we pass to the
func processCommand(funcs []test.FunctionDetails) error {
	t := setupTestContext()
	ctx := t.Context
	for _, f := range funcs {
		fmt.Println("  function ", f.Name)

		var err error
		ctx, err = t.CallFunction(f.Name, f.Parameters, ctx)
		if err != nil {
			fmt.Printf("unable to call function: %s\n", err)
		}
	}

	return nil
}

// http_post_func makes a http post
func http_post_func(p string, ctx context.Context) (context.Context, error) {
	var uri string
	paramsFromString(p, &uri)

	fmt.Println("calling function with uri: %s", uri)

	return ctx, nil
}
