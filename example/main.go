package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"

	"github.com/shipyard-run/hclconfig"
	"github.com/shipyard-run/hclconfig/types"
)

func main() {

	o := hclconfig.DefaultOptions()

	// set the callback that will be executed when a resource has been created
	// this function can be used to execute any external work required for the
	// resource.
	o.ParseCallback = func(r types.Resource) error {
		fmt.Printf("  resource '%s' named '%s' has been parsed from the file: %s\n", r.Metadata().Type, r.Metadata().Name, r.Metadata().File)
		return nil
	}

	p := hclconfig.NewParser(o)
	// register the types
	p.RegisterType("config", &Config{})
	p.RegisterType("postgres", &PostgreSQL{})

	// register a custom function
	p.RegisterFunction("random_number", func() (int, error) {
		return rand.Intn(100), nil
	})

	fmt.Println("## Parse the config")
	c, err := p.ParseFile("./config.hcl")
	if err != nil {
		fmt.Printf("An error occurred processing the config: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("")

	// print the config
	printConfig(c)
	fmt.Println("")

	// serialize the config to a file
	d, err := c.ToJSON()
	ioutil.WriteFile("./config.json", d, os.ModePerm)

	// deserialize the config
	nc, err := p.UnmarshalJSON(d)
	if err != nil {
		fmt.Printf("An error occurred unmarshalling the config: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("## Process config")
	nc.Process(func(r types.Resource) error {
		fmt.Println("  ", r.Metadata().ID)
		return nil
	}, false)

	fmt.Println("")
	fmt.Println("## Process config reverse")

	nc.Process(func(r types.Resource) error {
		fmt.Println("  ", r.Metadata().ID)
		return nil
	}, true)

}

func printConfig(c *hclconfig.Config) {
	fmt.Println("## Dump config")

	for _, r := range c.Resources {
		switch r.Metadata().Type {
		case "config":
			t := r.(*Config)
			fmt.Printf("  Config %s\n", t.Name)
			fmt.Printf("  Module %s\n", t.Module)
			fmt.Printf("  --- ID: %s\n", t.ID)
			fmt.Printf("  --- DBConnectionString: %s\n", t.DBConnectionString)
			fmt.Printf("  --- Timeouts\n")
			fmt.Printf("  ------ Connection: %d\n", t.Timeouts.Connection)
			fmt.Printf("  ------ KeepAlive: %d\n", t.Timeouts.KeepAlive)
			fmt.Printf("  ------ TLSHandshake: %d\n", t.Timeouts.TLSHandshake)

		case "postgres":
			t := r.(*PostgreSQL)
			fmt.Printf("  Postgres %s\n", t.Name)
			fmt.Printf("  Module %s\n", t.Module)
			fmt.Printf("  --- Location: %s\n", t.Location)
			fmt.Printf("  --- Port: %d\n", t.Port)
			fmt.Printf("  --- DBName: %s\n", t.DBName)
			fmt.Printf("  --- Username: %s\n", t.Username)
			fmt.Printf("  --- Password: %s\n", t.Password)
			fmt.Printf("  --- ConnectionString: %s\n", t.ConnectionString)

		case "output":
			t := r.(*types.Output)
			fmt.Printf("  Postgres %s\n", t.Name)
			fmt.Printf("  Module %s\n", t.Module)
			fmt.Printf("  --- Value: %s\n", t.Value)
		}

		fmt.Println("")
	}
}
