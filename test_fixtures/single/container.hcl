variable "cpu_resources" {
  default = 2048
}

resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}

resource "container" "consul" {

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resource.network.onprem.name
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

}

output "container_name" {
  value = resource.container.consul.name
}

output "container_resources_cpu" {
  value = resource.container.consul.resources.cpu
}
