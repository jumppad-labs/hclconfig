// Command plugin shows XCL used with plugins. It applies the same
// configuration and the same Go types as the configonly example, but the
// block types are provided by two plugins whose providers take part in the
// lifecycle, creating the resources on apply and destroying them at the end:
//
//   - ExamplePlugin (./internal) is an in-process plugin, compiled into this
//     program. It provides postgres, and fills in the computed
//     connection_string that the configuration only example leaves empty.
//   - external (./external) is an external plugin, compiled to its own binary
//     that xcl starts as a separate process and calls over gRPC. It provides
//     app.
//
// Build the external plugin and run the example from this directory with
// `make run`, see the Makefile for the other targets. The configuration
// directory and the external plugin binary can be passed as arguments:
// `go run . <config dir> <external plugin binary>`, they default to ../config
// and ./build/external.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/eventlog"
	"github.com/jumppad-labs/xcl/example/plugin/internal"
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

	externalPlugin := "./build/external"
	if len(os.Args) > 2 {
		externalPlugin = os.Args[2]
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	_, err = run(os.Stdout, logger.NewStdOutLogger(), dir, externalPlugin, filepath.Join(stateDir, "state.json"))
	os.RemoveAll(stateDir)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the in-process ExamplePlugin and
// the external plugin binary at externalPlugin registered, keeping the state
// in a file at statePath. It writes the resources and query results to out,
// then destroys everything through the providers and returns the resources
// that were applied. The plugins log to log, and every lifecycle event is
// logged to it too.
func run(out io.Writer, log logger.Logger, dir string, externalPlugin string, statePath string) ([]any, error) {
	r := registry.NewPluginRegistry(log)

	// The external plugin runs as a separate process, stop it when done
	defer func() {
		for _, host := range r.GetPluginHosts() {
			host.Stop()
		}
	}()

	// Register the in-process plugin, which provides every block type it
	// registered in Init
	if err := r.RegisterPlugin(&internal.ExamplePlugin{}); err != nil {
		return nil, err
	}

	// Start the external plugin binary and register the block types it
	// provides
	if err := r.RegisterPluginWithPath(externalPlugin); err != nil {
		return nil, fmt.Errorf("%w, build it with `make build` in example/plugin", err)
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

	// Plugin types are held as types generated from the plugin's schema, the
	// querier copies them into the Go type
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
