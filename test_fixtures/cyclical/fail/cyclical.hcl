resource "network" "one" {
  subnet = "10.0.0.1/16"
}

resource "network" "two" {
  // creates a cyclical reference
  subnet = resource.container.one.created_network_map.one.subnet
}

resource "container" "one" {
  network {
    name       = "one"
    ip_address = "127.0.0.1"
  }

  network {
    name       = "two"
    ip_address = "127.0.0.2"
  }

  created_network_map = {
    "one" = resource.network.one
    "two" = resource.network.two
  }
}