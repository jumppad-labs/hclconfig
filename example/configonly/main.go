// Command configonly shows XCL used for configuration only: parsing a
// configuration into Go objects. The block types are plain Go types
// registered on the plugin registry, there is no plugin and no provider.
// Blocks are decoded into the Go types, references between them are resolved,
// and no provider is ever called.
//
// The configuration it parses (./config) is a small Kubernetes-like
// deployment, and the Go types it parses into are in ./resources. Between
// them they show blocks nested inside blocks, blocks that repeat into a
// slice, and resources linked to each other by reference.
//
// The state is kept in a file, and after the resources are printed everything
// is destroyed again, which for these types only clears them from the state.
//
// Run it from this directory with `make run`, see the Makefile for the other
// targets. The configuration directory can be passed as an argument:
// `go run . <config dir>`, it defaults to ./config.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/configonly/resources"
	"github.com/jumppad-labs/xcl/example/eventlog"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

func main() {
	dir := "./config"
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

	// Register each Go type under the block type name used in configuration.
	// A registered type needs nothing else: no plugin, no provider, no schema
	// to write by hand.
	if err := r.RegisterType("config_map", &resources.ConfigMap{}); err != nil {
		return nil, err
	}

	if err := r.RegisterType("deployment", &resources.Deployment{}); err != nil {
		return nil, err
	}

	if err := r.RegisterType("service", &resources.Service{}); err != nil {
		return nil, err
	}

	if err := r.RegisterType("ingress", &resources.Ingress{}); err != nil {
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

	if err := printDeployments(out, c); err != nil {
		return nil, err
	}

	if err := printRouting(out, c); err != nil {
		return nil, err
	}

	applied := append([]any{}, c.GetResources()...)

	return applied, nil
}

// printDeployments writes each deployment and walks the blocks nested inside
// it, the containers and, for each of those, its ports, environment and
// resource limits. Registered types come back as the Go type that was
// registered, so the nested blocks are ordinary Go structs and slices.
func printDeployments(out io.Writer, c *xcl.Config) error {
	deployments, err := xcl.NewQuerier[resources.Deployment](c).FindResourcesByType("deployment")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Deployments")
	for _, d := range deployments {
		fmt.Fprintf(out, "  %s replicas=%d\n", d.Meta.ID, d.Replicas)

		for _, container := range d.Containers {
			fmt.Fprintf(out, "    container %s image=%s\n", container.Name, container.Image)

			for _, port := range container.Ports {
				fmt.Fprintf(out, "      port %s container_port=%d\n", port.Name, port.ContainerPort)
			}

			// env values were read from the config map, the reference is
			// resolved by the time the resource is returned
			for _, env := range container.Env {
				fmt.Fprintf(out, "      env %s=%s\n", env.Name, env.Value)
			}

			// A block that appears once is a pointer, nil when the
			// configuration leaves it out
			if container.Resources != nil {
				fmt.Fprintf(out, "      limits cpu=%s memory=%s\n", container.Resources.Limits.CPU, container.Resources.Limits.Memory)
				fmt.Fprintf(out, "      requests cpu=%s memory=%s\n", container.Resources.Requests.CPU, container.Resources.Requests.Memory)
			}

			for _, mount := range container.VolumeMounts {
				fmt.Fprintf(out, "      volume_mount %s path=%s\n", mount.Name, mount.Path)
			}
		}

		for _, volume := range d.Volumes {
			fmt.Fprintf(out, "    volume %s config_map=%s\n", volume.Name, volume.ConfigMap)
		}
	}

	return nil
}

// printRouting writes the two resources that are linked to the deployment,
// their values were read from the blocks they reference rather than repeated
// in the configuration
func printRouting(out io.Writer, c *xcl.Config) error {
	service, err := xcl.NewQuerier[resources.Service](c).FindResource("resource.service.api")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Service")
	fmt.Fprintf(out, "  %s deployment=%s port=%d target_port=%d\n", service.Meta.ID, service.Deployment, service.Port, service.TargetPort)

	ingress, err := xcl.NewQuerier[resources.Ingress](c).FindResource("resource.ingress.api")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Ingress")
	fmt.Fprintf(out, "  %s host=%s\n", ingress.Meta.ID, ingress.Host)

	for _, rule := range ingress.Rules {
		fmt.Fprintf(out, "    rule path=%s service=%s port=%d\n", rule.Path, rule.Service, rule.Port)
	}

	return nil
}
