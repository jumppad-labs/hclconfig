package internal

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// ExamplePlugin is an in-process plugin, it is compiled into this program and
// called directly. It provides the postgres block type, the app block type is
// provided by the external plugin in ./external.
type ExamplePlugin struct {
	plugins.PluginBase
}

// Ensure ExamplePlugin implements the Plugin interface
var _ plugins.Plugin = (*ExamplePlugin)(nil)

// Init registers the block types the plugin provides, with their providers.
// xcl tags everything the plugin logs with plugin=ExamplePlugin, and
// everything a provider logs with provider=<block type> as well.
func (p *ExamplePlugin) Init(logger logger.Logger, state plugins.State) error {
	logger.Debug("init")

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"postgres",
		&resources.PostgreSQL{},
		&postgresProvider{},
	)
}

// postgresProvider handles the lifecycle of postgres blocks. A real provider
// would create a database, this one only fills in the connection string.
//
// Change detection comes from the embedded DefaultChanged.
type postgresProvider struct {
	plugins.DefaultChanged[*resources.PostgreSQL]

	logger logger.Logger
}

var _ plugins.ResourceProvider[*resources.PostgreSQL] = (*postgresProvider)(nil)

// Init is called once when the provider is registered. The logger it receives
// writes to the host's logger, tagging every message with
// plugin=ExamplePlugin provider=postgres.
func (p *postgresProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	p.logger = logger
	p.logger.Debug("init")

	return nil
}

// Create sets the computed connection string, configured fields are never
// changed
func (p *postgresProvider) Create(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	db.ConnectionString = connectionString(db)
	p.logger.Debug("create", "id", db.Meta.ID, "connection_string", db.ConnectionString)

	return db, nil
}

// Read reports the database as configured, xcl has already carried the
// computed connection string over from the previous state
func (p *postgresProvider) Read(ctx context.Context, old *resources.PostgreSQL, new *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	p.logger.Debug("read", "id", new.Meta.ID)

	return new, nil
}

// Update sets the computed connection string for the changed configuration
func (p *postgresProvider) Update(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	db.ConnectionString = connectionString(db)
	p.logger.Debug("update", "id", db.Meta.ID, "connection_string", db.ConnectionString)

	return db, nil
}

// Destroy would remove the database, there is nothing to remove here
func (p *postgresProvider) Destroy(ctx context.Context, db *resources.PostgreSQL, force bool) error {
	p.logger.Debug("destroy", "id", db.Meta.ID, "force", force)

	return nil
}

func (p *postgresProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func connectionString(db *resources.PostgreSQL) string {
	return fmt.Sprintf("postgres://%s@%s:%d/%s", db.Username, db.Location, db.Port, db.DBName)
}
