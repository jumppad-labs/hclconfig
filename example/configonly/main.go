// Command configonly shows XCL used for configuration only. The block types
// are plain Go types registered on the plugin registry, there is no plugin and
// no provider: blocks are decoded into the Go types, references between them
// are resolved, and nothing is created, read or destroyed.
//
// Run it from this directory with `make run`, see the Makefile for the other
// targets. The configuration directory can be passed as an argument:
// `go run . <config dir>`, it defaults to ../config.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/eventlog"
	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
)

func main() {
	dir := "../config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	if _, err := run(os.Stdout, logger.NewStdOutLogger(), dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the example types registered,
// writes the resources and query results to out, and returns the resources.
// Every event xcl fires is logged to log.
func run(out io.Writer, log logger.Logger, dir string) ([]any, error) {
	r := registry.NewPluginRegistry(log)

	// Register each Go type under the block type name used in configuration
	if err := r.RegisterType("postgres", &resources.PostgreSQL{}); err != nil {
		return nil, err
	}

	if err := r.RegisterType("app", &resources.App{}); err != nil {
		return nil, err
	}

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(r),
		xcl.WithEventHandler(eventlog.Handler(log)),
	)

	if err := c.Apply(dir); err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Resources")
	for _, res := range c.GetResources() {
		meta, err := types.GetMeta(res)
		if err != nil {
			return nil, err
		}

		fmt.Fprintf(out, "  %s\n", meta.ID)
	}

	// Registered types come back as the Go type that was registered
	databases, err := xcl.NewQuerier[resources.PostgreSQL](c).FindResourcesByType("postgres")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Databases")
	for _, db := range databases {
		fmt.Fprintf(out, "  %s location=%s port=%d connection_string=%q\n", db.Meta.ID, db.Location, db.Port, db.ConnectionString)
	}

	app, err := xcl.NewQuerier[resources.App](c).FindResource("resource.app.web")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## App")
	fmt.Fprintf(out, "  %s database_location=%s database_user=%s analytics_location=%s connection_string=%q\n",
		app.Meta.ID, app.DatabaseLocation, app.DatabaseUser, app.AnalyticsLocation, app.ConnectionString)

	return c.GetResources(), nil
}
