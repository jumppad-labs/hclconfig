resource "network" "test" {
  subnet = "hello erik"
}

// These fixtures define the various types of expressions that can be used within
// hcl config
resource "container" "consul" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name = resource.network.test.nam // missing property
  }

  volume {
    source      = "images.volume.shipyard.run"
    destination = "/cache"
    type        = "volume"
  }

  volume {
    source      = "images.volume.jumppad.dev"
    destination = "/cache2"
    type        = "volume2"
  }
}