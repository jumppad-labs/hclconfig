package main

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/example/types"
	"github.com/jumppad-labs/hclconfig/test"
	htypes "github.com/jumppad-labs/hclconfig/types"
)

func main() {

	o := hclconfig.DefaultOptions()

	// set the callback that will be executed when a resource has been created
	// this function can be used to execute any external work required for the
	// resource.
	o.ParseCallback = func(r htypes.Resource) error {
		fmt.Printf("  resource '%s' named '%s' has been parsed from the file: %s\n", r.Metadata().Type, r.Metadata().Name, r.Metadata().File)
		return nil
	}

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
		return test.FunctionDetails{}, nil
	})

	p.RegisterFunction("http_post", func(uri string) (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
	})

	ferr := p.RegisterFunction("with_headers", func(headers map[string]string) (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
	})

	if ferr != nil {
		panic(ferr)
	}

	p.RegisterFunction("and_body", func(body string) (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
	})

	p.RegisterFunction("to_return_status_code", func(code int) (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
	})

	p.RegisterFunction("and", func() (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
	})

	p.RegisterFunction("the_body_contains", func(contents string) (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
	})

	p.RegisterFunction("body", func() (test.FunctionDetails, error) {
		return test.FunctionDetails{}, nil
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
