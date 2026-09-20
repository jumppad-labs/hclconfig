# Builds and runs the configuration only example. It uses no plugins, the
# block types are plain Go types registered with the plugin registry.

BUILD_DIR := build
CONFIG_DIR := ./config

.PHONY: run test

# Run the example against its configuration
run:
	go run . $(CONFIG_DIR)

# Run the example tests
test:
	go test -v ./...