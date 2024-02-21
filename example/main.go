package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/types"
)

func main() {

	o := hclconfig.DefaultOptions()

	// set the callback that will be executed when a resource has been created
	// this function can be used to execute any external work required for the
	// resource.
	o.Callback = func(r types.Resource) error {
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
	d, _ := c.ToJSON()
	err = os.WriteFile("./config.json", d, os.ModePerm)
	if err != nil {
		fmt.Println("unable to write config", err)
	}

	// deserialize the config
	nc, err := p.UnmarshalJSON(d)
	if err != nil {
		fmt.Printf("An error occurred unmarshalling the config: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("## Process config")
	nc.Walk(func(r types.Resource) error {
		fmt.Println("  ", r.Metadata().ID)
		return nil
	}, false)

	fmt.Println("")
	fmt.Println("## Process config reverse")

	nc.Walk(func(r types.Resource) error {
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
			fmt.Println(printConfigT(t, 2))

		case "postgres":
			t := r.(*PostgreSQL)
			fmt.Println(printPostgres(t, 2))

		case "output":
			t := r.(*types.Output)
			fmt.Printf("  Postgres %s\n", t.Meta.Name)
			fmt.Printf("  Module %s\n", t.Meta.Module)
			fmt.Printf("  --- Value: %s\n", t.Value)
		}

		fmt.Println("")
	}
}

func printConfigT(t *Config, indent int) string {
	str := bytes.NewBufferString("")
	pad := ""
	for i := 0; i < indent; i++ {
		pad += " "
	}

	fmt.Fprintf(str, "%sConfig %s\n", pad, t.Meta.Name)
	fmt.Fprintf(str, "%sModule %s\n", pad, t.Meta.Module)
	fmt.Fprintf(str, "%s--- ID: %s\n", pad, t.Meta.ID)
	fmt.Fprintf(str, "%s--- DBConnectionString: %s\n", pad, t.DBConnectionString)
	fmt.Fprintf(str, "%s--- Timeouts\n", pad)
	fmt.Fprintf(str, "%s------ Connection: %d\n", pad, t.Timeouts.Connection)
	fmt.Fprintf(str, "%s------ KeepAlive: %d\n", pad, t.Timeouts.KeepAlive)
	fmt.Fprintf(str, "%s------ TLSHandshake: %d\n", pad, t.Timeouts.TLSHandshake)
	fmt.Fprintf(str, "%s--- MainDBConnection:\n", pad)

	fmt.Fprintf(str, "%s", printPostgres(&t.MainDBConnection, 8))

	for i, p := range t.OtherDBConnections {
		fmt.Fprintf(str, "%s--- OtherDBConnections[%d]:\n", pad, i)
		fmt.Fprintf(str, "%s", printPostgres(&p, 8))
	}

	return str.String()
}

func printPostgres(p *PostgreSQL, indent int) string {
	str := bytes.NewBufferString("")
	pad := ""
	for i := 0; i < indent; i++ {
		pad += " "
	}

	fmt.Fprintf(str, "%sPostgres %s\n", pad, p.Meta.Name)
	fmt.Fprintf(str, "%sModule %s\n", pad, p.Meta.Module)
	fmt.Fprintf(str, "%s--- Location: %s\n", pad, p.Location)
	fmt.Fprintf(str, "%s--- Port: %d\n", pad, p.Port)
	fmt.Fprintf(str, "%s--- DBName: %s\n", pad, p.DBName)
	fmt.Fprintf(str, "%s--- Username: %s\n", pad, p.Username)
	fmt.Fprintf(str, "%s--- Password: %s\n", pad, p.Password)
	fmt.Fprintf(str, "%s--- ConnectionString: %s\n", pad, p.ConnectionString)

	return str.String()
}
