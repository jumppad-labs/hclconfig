// Command plugin shows XCL used with a full plugin. It applies the same
// configuration and the same Go types as the configonly example, but the
// block types are provided by an in-process plugin whose providers take part
// in the lifecycle: the postgres provider fills in the computed
// connection_string when a database is created, which the configuration only
// example leaves empty.
//
// Run it from this directory with `go run .`, or pass the configuration
// directory, i.e. `go run ./example/plugin ./example/config`.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
)

// ExamplePlugin provides the postgres and app block types, each with a
// provider that handles its lifecycle
type ExamplePlugin struct {
	plugins.PluginBase
}

// Ensure ExamplePlugin implements the Plugin interface
var _ plugins.Plugin = (*ExamplePlugin)(nil)

// Init registers the block types the plugin provides, with their providers
func (p *ExamplePlugin) Init(logger logger.Logger, state plugins.State) error {
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"postgres",
		&resources.PostgreSQL{},
		&postgresProvider{},
	)
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"app",
		&resources.App{},
		&appProvider{},
	)
}

// postgresProvider handles the lifecycle of postgres blocks. A real provider
// would create a database, this one only fills in the connection string.
type postgresProvider struct {
	plugins.DefaultChanged[*resources.PostgreSQL]
}

var _ plugins.ResourceProvider[*resources.PostgreSQL] = (*postgresProvider)(nil)

func (p *postgresProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

// Create sets the computed connection string, configured fields are never
// changed
func (p *postgresProvider) Create(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	db.ConnectionString = connectionString(db)
	return db, nil
}

// Read reports the database as configured, xcl has already carried the
// computed connection string over from the previous state
func (p *postgresProvider) Read(ctx context.Context, old *resources.PostgreSQL, new *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	return new, nil
}

// Update sets the computed connection string for the changed configuration
func (p *postgresProvider) Update(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	db.ConnectionString = connectionString(db)
	return db, nil
}

func (p *postgresProvider) Destroy(ctx context.Context, db *resources.PostgreSQL, force bool) error {
	return nil
}

func (p *postgresProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func connectionString(db *resources.PostgreSQL) string {
	return fmt.Sprintf("postgres://%s@%s:%d/%s", db.Username, db.Location, db.Port, db.DBName)
}

// appProvider handles the lifecycle of app blocks, there is nothing to create
// so every call succeeds without changing the app
type appProvider struct {
	plugins.DefaultChanged[*resources.App]
}

var _ plugins.ResourceProvider[*resources.App] = (*appProvider)(nil)

func (p *appProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *appProvider) Create(ctx context.Context, app *resources.App) (*resources.App, error) {
	return app, nil
}

func (p *appProvider) Read(ctx context.Context, old *resources.App, new *resources.App) (*resources.App, error) {
	return new, nil
}

func (p *appProvider) Update(ctx context.Context, app *resources.App) (*resources.App, error) {
	return app, nil
}

func (p *appProvider) Destroy(ctx context.Context, app *resources.App, force bool) error {
	return nil
}

func (p *appProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func main() {
	dir := "../config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	if _, err := run(os.Stdout, dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the example plugin registered,
// writes the resources and query results to out, and returns the resources
func run(out io.Writer, dir string) ([]any, error) {
	r := registry.NewPluginRegistry(logger.NewStdOutLogger())

	// Register the plugin, which provides every block type it registered in Init
	if err := r.RegisterPlugin(&ExamplePlugin{}); err != nil {
		return nil, err
	}

	c := xcl.NewConfig(xcl.WithPluginRegistry(r))

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

	return c.GetResources(), nil
}
