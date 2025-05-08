variable "disabled" {
  default = true
}

resource "network" "onprem" {
  disabled = variable.disabled
  subnet = "0.0.0.0/24"
}

## This resource will not have it's dependencies
## validated as disabled can be calculated at parse time
resource "container" "disabled_value" {
  disabled = true

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

## This resource will have it's dependencies
## validated as disabled can not be calculated at parse time
## since the `disable` field is set by a variable, this is resolved
## later in the process
resource "container" "disabled_variable" {
  disabled = variable.disabled

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  // if not removed, will fail due to dependency violation
  //network {
  //  name       = resource.network.onprem.name
  //  ip_address = "10.6.0.200"
  //}

  dns = ["a", "b", "c"]

  resources {
    memory  = 1024
    cpu_pin = [1]
  }
}

## This resource sets disabled based on another resource
resource "container" "disabled_variable_deps" {
  disabled = resource.container.disabled_variable.disabled

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  // if not removed, will fail due to dependency violation
  //network {
  //  name       = resource.network.onprem.name
  //  ip_address = "10.6.0.200"
  //}

  dns = ["a", "b", "c"]

  resources {
    memory  = 1024
    cpu_pin = [1]
  }
}