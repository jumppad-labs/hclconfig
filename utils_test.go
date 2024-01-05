package hclconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadResourceFromFileAtLocation(t *testing.T) {
	c, err := ReadFileLocation("./test_fixtures/single/container.hcl", 9, 1, 29, 2)
	require.NoError(t, err)

	// check the contents
	require.Equal(t, container, c)
}

func TestLineFromFileAtLocation(t *testing.T) {
	c, err := ReadFileLocation("./test_fixtures/single/container.hcl", 9, 1, 9, 30)
	require.NoError(t, err)

	// check the contents
	require.Equal(t, singleLine, c)
}

func TestCalculatesHashFromString(t *testing.T) {
	h := HashString("Hello World")

	require.Equal(t, h, "b10a8db164e0754105b7a99be72e3fe5")
}

var singleLine = `resource "container" "consul"`

var container = `resource "container" "consul" {

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resource.network.onprem.resource_name
    ip_address = "10.6.0.200"
  }

  resources {
    # Max CPU to consume, 1024 is one core, default unlimited
    cpu = variable.cpu_resources
  }

  volume {
    source      = "images.volume.shipyard.run"
    destination = "/cache"
    type        = "volume"
  }

}`
