package main

import (
	"fmt"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

func main() {
	os.RemoveAll(".xcl")

	fmt.Println("## XCL Config Example")
	fmt.Println("## Parsing config.hcl...\n")

	// Create a plugin registry
	log := logger.NewStdOutLogger()
	pluginRegistry := registry.NewPluginRegistry(log)

	// Create config with options
	// Note: StateStore not configured for this example
	config := xcl.NewConfig(
		xcl.WithPluginRegistry(pluginRegistry),
	)

	// Apply the configuration
	err := config.Apply("./config.hcl")
	if err != nil {
		fmt.Printf("An error occurred processing the config: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("## Configuration applied successfully!\n")

	// Demonstrate querying resources
	demonstrateQuerying(config)
}

func demonstrateQuerying(config *xcl.Config) {
	fmt.Println("## Querying Resources")
	fmt.Printf("Total resources: %d\n\n", config.ResourceCount())

	// Get all resources
	resources := config.GetResources()
	fmt.Printf("Found %d resources:\n", len(resources))
	for i, r := range resources {
		fmt.Printf("  %d. %+v\n", i+1, r)
	}
	fmt.Println("")

	// Create a strongly-typed querier for PostgreSQL resources
	querier := xcl.NewQuerier[PostgreSQL](config)

	// Find all PostgreSQL resources by type
	databases, err := querier.FindResourcesByType()
	if err == nil && len(databases) > 0 {
		fmt.Printf("Found %d PostgreSQL resources:\n", len(databases))
		for _, db := range databases {
			fmt.Printf("  - %s (location: %s, port: %d)\n", db.Meta.Name, db.Location, db.Port)
		}
		fmt.Println("")
	} else if err != nil {
		fmt.Printf("No PostgreSQL resources found or error: %s\n", err)
	}

	// Demonstrate diff with Validate
	fmt.Println("## Demonstrating Validate (dry-run)")
	diff, err := config.Validate("./config.hcl")
	if err != nil {
		fmt.Printf("Validation error: %s\n", err)
		return
	}

	fmt.Printf("Changes that would be applied:\n")
	fmt.Printf("  - To Create: %d resources\n", len(diff.ToCreate))
	fmt.Printf("  - To Update: %d resources\n", len(diff.ToUpdate))
	fmt.Printf("  - To Destroy: %d resources\n", len(diff.ToDestroy))
	fmt.Println("")
}
