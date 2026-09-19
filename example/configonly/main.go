// Command configonly shows XCL used for configuration only. The block types
// are plain Go types registered on the plugin registry, there is no plugin and
// no provider: blocks are decoded into the Go types, references between them
// are resolved, and no provider is ever called. The state is kept in a file,
// and after the resources are printed everything is destroyed again, which
// for these types only clears them from the state.
//
// Run it from this directory with `make run`, see the Makefile for the other
// targets. The configuration directory can be passed as an argument:
// `go run . <config dir>`, it defaults to ../config.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/eventlog"
	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

func main() {
	dir := "../config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	_, err = run(os.Stdout, logger.NewStdOutLogger(), dir, filepath.Join(stateDir, "state.json"))
	os.RemoveAll(stateDir)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the example types registered,
// keeping the state in a file at statePath, writes the resources and query
// results to out, then destroys everything and returns the resources that were
// applied. Every event xcl fires is logged to log.
func run(out io.Writer, log logger.Logger, dir string, statePath string) ([]any, error) {
	r := registry.NewPluginRegistry(log)

	// Register each Go type under the block type name used in configuration
	if err := r.RegisterType("postgres", &resources.PostgreSQL{}); err != nil {
		return nil, err
	}

	if err := r.RegisterType("app", &resources.App{}); err != nil {
		return nil, err
	}

	// Keep the state in a file, Destroy works from it alone
	store, err := state.NewFileStateStore(statePath, r)
	if err != nil {
		return nil, err
	}

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(r),
		xcl.WithStateStore(store),
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

	applied := append([]any{}, c.GetResources()...)

	// Destroy everything that was applied, dependents before what they depend
	// on, working only from the saved state
	if err := c.Destroy(); err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Destroyed")
	fmt.Fprintf(out, "  %d resources remaining\n", c.ResourceCount())

	return applied, nil
}
