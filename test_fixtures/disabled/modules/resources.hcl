variable "disable_resources" {
  default = false
}

variable "is_true" {
  default = true
}

resource "network" "dependent" {
  disabled = true
  subnet   = "0.0.0.0/24"
}

resource "network" "onprem" {
  disabled = variable.disable_resources == resource.network.dependent.disabled
  subnet   = "0.0.0.0/24"
}

resource "container" "enabled" {
  disabled = variable.disable_resources

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resource.network.onprem.meta.name
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
