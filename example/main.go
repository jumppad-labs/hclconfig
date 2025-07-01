package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/internal/resources"
	"github.com/jumppad-labs/hclconfig/types"
)

func main() {

	o := hclconfig.DefaultOptions()


	// Configure plugin discovery
	// By default, plugins are auto-discovered from:
	// - ./.hclconfig/plugins/
	// - ~/.hclconfig/plugins/
	// - Directories in HCLCONFIG_PLUGIN_PATH environment variable
	//
	// You can customize this behavior:
	// o.AutoDiscoverPlugins = false  // Disable auto-discovery
	// o.PluginDirectories = append(o.PluginDirectories, "/custom/plugin/path")
	// o.PluginNamePattern = "my-plugin-*"  // Change pattern (default: "hclconfig-plugin-*")
	
	// Add a logger to see plugin discovery in action
	o.Logger = func(msg string) {
		fmt.Printf("[Plugin Discovery] %s\n", msg)
	}

	// o.PrimativesOnly = true

	p := hclconfig.NewParser(o)
	
	// You can still manually register plugins alongside auto-discovery
	// This in-process plugin will be registered in addition to any discovered plugins
	examplePlugin := &ExamplePlugin{}
	err := p.RegisterPlugin(examplePlugin)
	if err != nil {
		fmt.Printf("Failed to register example plugin: %s\n", err)
		os.Exit(1)
	}
	
	// You can also manually register external plugin binaries
	// This is useful for plugins that aren't in the standard discovery directories
	// err = p.RegisterPluginWithPath("/path/to/external/plugin")
	// if err != nil {
	//     fmt.Printf("Failed to register external plugin: %s\n", err)
	// }

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

	// demonstrate state store usage
	// create a resource registry with the same plugin hosts as the parser
	registry := hclconfig.NewResourceRegistry([]plugins.PluginHost{})
	stateStore := hclconfig.NewFileStateStore("./example-state", registry)
	
	// save the parsed config
	err = stateStore.Save(c)
	if err != nil {
		fmt.Println("unable to save config", err)
	}

	// load the config from state
	nc, err := stateStore.Load()
	if err != nil {
		fmt.Printf("An error occurred loading the config: %s\n", err)
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
			t := r.(*resources.Output)
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

// ExamplePlugin provides the Config and PostgreSQL resource types for the example
type ExamplePlugin struct {
	plugins.PluginBase
}

// Init initializes the example plugin
func (p *ExamplePlugin) Init(logger plugins.Logger, state plugins.State) error {
	// Register Config resource
	configResource := &Config{}
	configProvider := &ExampleResourceProvider[*Config]{}
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"config", 
		configResource,
		configProvider,
	)
	if err != nil {
		return err
	}

	// Register PostgreSQL resource
	postgresResource := &PostgreSQL{}
	postgresProvider := &ExampleResourceProvider[*PostgreSQL]{}
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"postgres",
		postgresResource,
		postgresProvider,
	)
}

// ExampleResourceProvider is a generic provider for example resources
type ExampleResourceProvider[T types.Resource] struct {
	logger    plugins.Logger
	state     plugins.State  
	functions plugins.ProviderFunctions
}

// Init initializes the provider
func (p *ExampleResourceProvider[T]) Init(state plugins.State, functions plugins.ProviderFunctions, logger plugins.Logger) error {
	p.state = state
	p.functions = functions
	p.logger = logger
	return nil
}

// Create is a no-op for the example
func (p *ExampleResourceProvider[T]) Create(ctx context.Context, resource T) (T, error) {
	return resource, nil
}

// Destroy is a no-op for the example
func (p *ExampleResourceProvider[T]) Destroy(ctx context.Context, resource T, force bool) error {
	return nil
}

// Refresh is a no-op for the example  
func (p *ExampleResourceProvider[T]) Refresh(ctx context.Context, resource T) error {
	return nil
}

// Changed always returns false for the example
func (p *ExampleResourceProvider[T]) Changed(ctx context.Context, resource T) (bool, error) {
	return false, nil
}

// Functions returns no functions
func (p *ExampleResourceProvider[T]) Functions() plugins.ProviderFunctions {
	return p.functions
}
