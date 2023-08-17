variable "consul_dc1_version" {
  default = "1.8.0"
}

variable "consul_dc2_version" {
  default = "2.8.0"
}

variable "cpu_resources" {
  default = 2048
}

resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}

resource "container" "consul_dc1" {
  image {
    name = "consul:${variable.consul_version}"
  }

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

resource "container" "consul_dc2" {

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
  value = resource.container.consul_dc1.name
}

output "container_resources_cpu" {
  value = resource.container.consul_dc2.resources.cpu
}

output "combined_list" {
  value = [
    resource.container.consul_dc1.resources.memory,
    resource.container.consul_dc1.resources.cpu
  ]
}

output "combined_map" {
  value = {
    name = resource.container.consul_dc2.name
    cpu  = resource.container.consul_dc2.resources.cpu
  }
}