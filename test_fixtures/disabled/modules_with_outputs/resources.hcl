resource "container" "service" {
  command = ["tail", "-f", "/dev/null"]

  network {
    name = "main"
  }

  dns = ["a", "b", "c"]

  resources {
    memory  = 1024
    cpu_pin = [1]
  }
}

output "service_address" {
  value = resource.container.service.network[0].name
}
