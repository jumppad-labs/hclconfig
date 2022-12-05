package main

import (
	"fmt"
	"os"

	"github.com/shipyard-run/hclconfig"
	"github.com/shipyard-run/hclconfig/types"
)

func main() {

	o := hclconfig.DefaultOptions()

	// set the environment variable prefix
	o.VariableEnvPrefix = "HCL_"

	p := hclconfig.NewParser(o)
	// register the types
	p.RegisterType("config", &Config{})
	p.RegisterType("postgres", &PostgreSQL{})

	c := hclconfig.NewConfig()

	var err error
	_, err = p.ParseFile("./config.hcl", c)
	if err != nil {
		fmt.Printf("An error occurred processing the config: %s", err)
		os.Exit(1)
	}

	// print the config
	printConfig(c)
}

func printConfig(c *hclconfig.Config) {
	for _, r := range c.Resources {
		switch r.Info().Type {
		case types.ResourceType("config"):
			t := r.(*Config)
			fmt.Printf("Config %s\n", t.Info().Name)
			fmt.Printf("--- ID: %s\n", t.ID)
			fmt.Printf("--- DBConnectionString: %s\n", t.DBConnectionString)
			fmt.Printf("--- Timeouts\n")
			fmt.Printf("------ Connection: %d\n", t.Timeouts.Connection)
			fmt.Printf("------ KeepAlive: %d\n", t.Timeouts.KeepAlive)
			fmt.Printf("------ TLSHandshake: %d\n", t.Timeouts.TLSHandshake)

		case types.ResourceType("postgres"):
			t := r.(*PostgreSQL)
			fmt.Printf("Postgres %s\n", t.Info().Name)
			fmt.Printf("--- Location: %s\n", t.Location)
			fmt.Printf("--- Port: %d\n", t.Port)
			fmt.Printf("--- DBName: %s\n", t.DBName)
			fmt.Printf("--- Username: %s\n", t.Username)
			fmt.Printf("--- Password: %s\n", t.Password)
			fmt.Printf("--- ConnectionString: %s\n", t.ConnectionString)
		}
	}
}
