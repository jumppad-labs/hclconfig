// Command external is an external plugin for the plugin example. It is
// compiled to its own binary, which xcl starts as a separate process and
// talks to over gRPC. It provides the app and ingress block types.
//
// Build it from the example/plugin directory with `make build`, which runs
// `go build -o build/external ./external`.
package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/xcl/example/plugin/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// ExternalPlugin provides the app and ingress block types
type ExternalPlugin struct {
	plugins.PluginBase
}

// Ensure ExternalPlugin implements the Plugin interface
var _ plugins.Plugin = (*ExternalPlugin)(nil)

// Init registers the block types the plugin provides, with their providers.
// An external plugin registers several block types exactly as an in-process
// one does, calling RegisterResourceProvider once per type.
//
// An external plugin is initialized when its process starts, before it is
// connected to the host, so logger is nil here. The providers are given a
// logger connected to the host before each call.
func (p *ExternalPlugin) Init(logger logger.Logger, state plugins.State) error {
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"app",
		&resources.App{},
		&appProvider{},
	)
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"ingress",
		&resources.Ingress{},
		&ingressProvider{},
	)
}

// appProvider handles the lifecycle of app blocks, the only thing it creates
// is the computed url, which the ingress block reads
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
// connection strings the in-process postgres and redis providers filled in,
// and sets the computed url that the ingress block reads
func (p *appProvider) Create(ctx context.Context, app *resources.App) (*resources.App, error) {
	app.URL = appURL(app)
	p.logger.Debug("", "event", "create", "resource", app.Meta.ID,
		"connection_string", app.ConnectionString, "cache_connection_string", app.CacheConnectionString, "url", app.URL)

	return app, nil
}

func (p *appProvider) Read(ctx context.Context, old *resources.App, new *resources.App) (*resources.App, error) {
	p.logger.Debug("", "event", "read", "resource", new.Meta.ID)

	return new, nil
}

func (p *appProvider) Update(ctx context.Context, app *resources.App) (*resources.App, error) {
	app.URL = appURL(app)
	p.logger.Debug("", "event", "update", "resource", app.Meta.ID, "url", app.URL)

	return app, nil
}

func (p *appProvider) Destroy(ctx context.Context, app *resources.App, force bool) error {
	p.logger.Debug("", "event", "destroy", "resource", app.Meta.ID, "force", force)

	return nil
}

func (p *appProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func appURL(app *resources.App) string {
	return fmt.Sprintf("http://%s", app.Meta.Name)
}

// ingressProvider handles the lifecycle of ingress blocks, the second block
// type this plugin provides. It is a separate provider with its own logger,
// tagged provider=ingress, registered in Init alongside the app one.
//
// Change detection comes from the embedded DefaultChanged.
type ingressProvider struct {
	plugins.DefaultChanged[*resources.Ingress]

	logger logger.Logger
}

var _ plugins.ResourceProvider[*resources.Ingress] = (*ingressProvider)(nil)

// Init stores the logger, which tags every message with plugin=external
// provider=ingress. As with the app provider, the first call, when the
// process starts, has no logger.
func (p *ingressProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	p.logger = logger

	return nil
}

// Create receives the ingress with the computed url of the app it routes to,
// filled in by the app provider in this same plugin
func (p *ingressProvider) Create(ctx context.Context, ingress *resources.Ingress) (*resources.Ingress, error) {
	p.logger.Debug("", "event", "create", "resource", ingress.Meta.ID, "hostname", ingress.Hostname, "app_url", ingress.AppURL)

	return ingress, nil
}

func (p *ingressProvider) Read(ctx context.Context, old *resources.Ingress, new *resources.Ingress) (*resources.Ingress, error) {
	p.logger.Debug("", "event", "read", "resource", new.Meta.ID)

	return new, nil
}

func (p *ingressProvider) Update(ctx context.Context, ingress *resources.Ingress) (*resources.Ingress, error) {
	p.logger.Debug("", "event", "update", "resource", ingress.Meta.ID)

	return ingress, nil
}

func (p *ingressProvider) Destroy(ctx context.Context, ingress *resources.Ingress, force bool) error {
	p.logger.Debug("", "event", "destroy", "resource", ingress.Meta.ID, "force", force)

	return nil
}

func (p *ingressProvider) Functions() plugins.ProviderFunctions {
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
