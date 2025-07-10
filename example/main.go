package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/types"
)

func main() {
	// Parse command line flags
	var format = flag.String("format", "table", "Output format: table, tree, card, json, or all")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nDemo of HCL config parser with pretty printer\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -format=table    # Show resources in table format\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -format=tree     # Show resources in tree format\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -format=all      # Show all formats\n", os.Args[0])
	}
	flag.Parse()

	// Validate format
	validFormats := map[string]hclconfig.PrintFormat{
		"table": hclconfig.FormatTable,
		"tree":  hclconfig.FormatTree,
		"card":  hclconfig.FormatCard,
		"json":  hclconfig.FormatJSON,
	}

	var selectedFormat hclconfig.PrintFormat
	var showAll bool
	if *format == "all" {
		showAll = true
	} else if f, ok := validFormats[*format]; ok {
		selectedFormat = f
	} else {
		fmt.Printf("Invalid format '%s'. Valid options: table, tree, card, json, all\n", *format)
		os.Exit(1)
	}

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
	o.Logger = &plugins.TestLogger{}

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

	// demonstrate the new pretty printer
	fmt.Printf("## Pretty Printer Demo (%s format)\n", *format)
	printer := hclconfig.NewResourcePrinter()

	// Find a resource to demonstrate
	if len(c.Resources) > 0 {
		if showAll {
			// Show all formats
			firstResource := c.Resources[0]

			fmt.Println("### Table Format:")
			printer.PrintResource(firstResource, hclconfig.FormatTable)
			fmt.Println("")

			fmt.Println("### Tree Format:")
			printer.PrintResource(firstResource, hclconfig.FormatTree)
			fmt.Println("")

			fmt.Println("### Card Format:")
			printer.PrintResource(firstResource, hclconfig.FormatCard)
			fmt.Println("")

			fmt.Println("### JSON Format:")
			printer.PrintResource(firstResource, hclconfig.FormatJSON)
			fmt.Println("")

			// Show multiple resources in table format
			if len(c.Resources) > 1 {
				fmt.Println("### All Resources (Table Format):")
				printer.PrintResources(c.Resources, hclconfig.FormatTable)
				fmt.Println("")
			}
		} else {
			// Show only selected format
			if len(c.Resources) > 1 {
				// Print all resources in selected format
				printer.PrintResources(c.Resources, selectedFormat)
			} else {
				// Print single resource
				printer.PrintResource(c.Resources[0], selectedFormat)
			}
		}
	}

	fmt.Println("")

	// demonstrate state store usage
	// create a resource registry with the same plugin hosts as the parser
	//registry := p.GetPluginRegistry()
	//stateStore := hclconfig.NewFileStateStore("./example-state", registry)

	//// save the parsed config
	//err = stateStore.Save(c)
	//if err != nil {
	//	fmt.Println("unable to save config", err)
	//}

	//// load the config from state
	//nc, err := stateStore.Load()
	//if err != nil {
	//	fmt.Printf("An error occurred loading the config: %s\n", err)
	//	os.Exit(1)
	//}

	//fmt.Println("## Process config")
	//nc.Walk(func(r types.Resource) error {
	//	fmt.Println("  ", r.Metadata().ID)
	//	return nil
	//}, false)

	//fmt.Println("")
	//fmt.Println("## Process config reverse")

	//nc.Walk(func(r types.Resource) error {
	//	fmt.Println("  ", r.Metadata().ID)
	//	return nil
	//}, true)

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
	p.logger.Info("Creating resource", "type", resource.Metadata().Type, "id", resource.Metadata().ID)
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

// Update is a no-op for the example
func (p *ExampleResourceProvider[T]) Update(ctx context.Context, resource T) error {
	return nil
}

// Changed always returns false for the example
func (p *ExampleResourceProvider[T]) Changed(ctx context.Context, old T, new T) (bool, error) {
	return false, nil
}

// Functions returns no functions
func (p *ExampleResourceProvider[T]) Functions() plugins.ProviderFunctions {
	return p.functions
}
