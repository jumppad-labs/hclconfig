package main

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/shipyard-run/hclconfig"
)

func main() {

	o := hclconfig.DefaultOptions()

	// set the environment variable prefix
	o.VariableEnvPrefix = "HCL_"

	p := hclconfig.NewParser(o)
	// register the types
	p.RegisterType("config", &Config{})
	p.RegisterType("postgres", &PostgreSQL{})

	// register a custom function
	p.RegisterFunction("random_number", func() (int, error) {
		return rand.Intn(100), nil
	})

	c := hclconfig.NewConfig()

	err := p.ParseFile("./config.hcl", c)
	if err != nil {
		fmt.Printf("An error occurred processing the config: %s", err)
		os.Exit(1)
	}

	// print the config
	printConfig(c)
}

func printConfig(c *hclconfig.Config) {
	for _, r := range c.Resources {
		switch r.Metadata().Type {
		case "config":
			t := r.(*Config)
			fmt.Printf("Config %s\n", t.Name)
			fmt.Printf("--- ID: %s\n", t.ID)
			fmt.Printf("--- DBConnectionString: %s\n", t.DBConnectionString)
			fmt.Printf("--- Timeouts\n")
			fmt.Printf("------ Connection: %d\n", t.Timeouts.Connection)
			fmt.Printf("------ KeepAlive: %d\n", t.Timeouts.KeepAlive)
			fmt.Printf("------ TLSHandshake: %d\n", t.Timeouts.TLSHandshake)

		case "postgres":
			t := r.(*PostgreSQL)
			fmt.Printf("Postgres %s\n", t.Name)
			fmt.Printf("--- Location: %s\n", t.Location)
			fmt.Printf("--- Port: %d\n", t.Port)
			fmt.Printf("--- DBName: %s\n", t.DBName)
			fmt.Printf("--- Username: %s\n", t.Username)
			fmt.Printf("--- Password: %s\n", t.Password)
			fmt.Printf("--- ConnectionString: %s\n", t.ConnectionString)
		}
	}
}
