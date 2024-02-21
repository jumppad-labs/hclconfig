variable "network" {
  default = "onprem"
}

resource "container" "consul" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = variable.network
    ip_address = "10.6.0.200"
  }

  volume {
    source      = "images.volume.shipyard.run"
    destination = "/cache"
    type        = "volume"
  }
}

local "test" {

  //value = resource.container.consul.name
  value = resource.container.consul.meta.name == "consul" ? "yes" : "no"
}

resource "template" "consul_config_update" {
  disabled = false

  source = resource.container.consul.meta.name == "consul" ? "yes" : "no"

  destination = "./consul.hcl"

  vars = {
    data_dir = "/tmp"
  }
}

resource "template" "consul_config_update2" {
  disabled = false

  source = local.test

  destination = "./consul.hcl"

  vars = {
    data_dir = "/tmp"
  }
}