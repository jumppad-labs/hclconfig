# Builds and runs the plugin example. The in-process plugin (internal/) is
# compiled into the example, the external plugin (external/) is compiled to its
# own binary, which the example starts as a separate process.

BUILD_DIR := build
EXTERNAL_PLUGIN := $(BUILD_DIR)/external
CONFIG_DIR := ./config

.PHONY: build run test clean

# Build the external plugin binary
build:
	go build -o $(EXTERNAL_PLUGIN) ./external

# Build the external plugin, then run the example against the shared configuration
run: build
	go run . $(CONFIG_DIR) $(EXTERNAL_PLUGIN)

# Run the example tests, they build the external plugin themselves
test:
	go test -v ./...

# Remove the built external plugin
clean:
	rm -rf $(BUILD_DIR)
