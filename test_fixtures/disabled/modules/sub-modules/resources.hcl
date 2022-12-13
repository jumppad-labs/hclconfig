container "enabled"{
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resource.network.onprem.name
    ip_address = "10.6.0.200"
  }

  dns = ["a","b","c"]

  resources {
    memory = 1024
    cpu_pin = [1]
  }
}
