// Command external is an external plugin for the plugin example. It is
// compiled to its own binary, which xcl starts as a separate process and
// talks to over gRPC. It provides the app block type.
//
// Build it from the example/plugin directory with `make build`, which runs
// `go build -o build/external ./external`.
package main

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/xcl/example/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// ExternalPlugin provides the app block type
type ExternalPlugin struct {
	plugins.PluginBase
}

// Ensure ExternalPlugin implements the Plugin interface
var _ plugins.Plugin = (*ExternalPlugin)(nil)

// Init registers the block types the plugin provides, with their providers.
//
// An external plugin is initialized when its process starts, before it is
// connected to the host, so logger is nil here. The providers are given a
// logger connected to the host before each call.
func (p *ExternalPlugin) Init(logger logger.Logger, state plugins.State) error {
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

// appProvider handles the lifecycle of app blocks, there is nothing to create
// so every call succeeds without changing the app
//
// Change detection comes from the embedded DefaultChanged.
type appProvider struct {
	plugins.DefaultChanged[*resources.App]

	logger logger.Logger
}

var _ plugins.ResourceProvider[*resources.App] = (*appProvider)(nil)

// Init stores the logger. In an external plugin Init is called again before
// every call from the host, with a logger connected to the host that tags
// every message with plugin=external provider=app, so it does not log
// itself. The first call, when the process starts, has no logger.
func (p *appProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	p.logger = logger

	return nil
}

// Create receives the app with every reference resolved, including the
// connection string the in-process postgres provider filled in
func (p *appProvider) Create(ctx context.Context, app *resources.App) (*resources.App, error) {
	p.logger.Debug("create", "id", app.Meta.ID, "connection_string", app.ConnectionString)

	return app, nil
}

func (p *appProvider) Read(ctx context.Context, old *resources.App, new *resources.App) (*resources.App, error) {
	p.logger.Debug("read", "id", new.Meta.ID)

	return new, nil
}

func (p *appProvider) Update(ctx context.Context, app *resources.App) (*resources.App, error) {
	p.logger.Debug("update", "id", app.Meta.ID)

	return app, nil
}

func (p *appProvider) Destroy(ctx context.Context, app *resources.App, force bool) error {
	p.logger.Debug("destroy", "id", app.Meta.ID, "force", force)

	return nil
}

func (p *appProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// main serves the plugin to the host that started this process
func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"plugin": &plugins.GRPCPlugin{
				Impl: &ExternalPlugin{},
			},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
