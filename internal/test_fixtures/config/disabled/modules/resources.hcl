variable "disable_resources" {
  default = false
}

resource "network" "onprem" {
  disabled = variable.disable_resources
  subnet = "0.0.0.0/24"
}

resource "container" "enabled" {
  disabled = variable.disable_resources

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resource.network.onprem.resource_name
    ip_address = "10.6.0.200"
  }

  dns = ["a", "b", "c"]

  resources {
    memory  = 1024
    cpu_pin = [1]
  }
}

module "sub" {
  disabled = variable.disable_resources

  source = "./sub-modules"
}
