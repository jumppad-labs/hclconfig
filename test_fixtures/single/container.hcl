variable "cpu_resources" {
  default = 2048
}

container "consul" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resources.network.onprem.name
    ip_address = "10.6.0.200"
  }

  dns = resources.container.base.dns

  resources {
    # Max CPU to consume, 1024 is one core, default unlimited
    cpu = var.cpu_resources
    # Pin container to specified CPU cores, default all cores
    cpu_pin = resources.container.base.resources.cpu_pin
    # max memory in MB to consume, default unlimited
    memory = resources.container.base.resources.memory
  }

  volume {
    source      = "."
    destination = "/test/${resources.template.consul_config.destination}"
  }
  

  volume {
    source      = resources.template.consul_config.destination
    destination = "/config/config.hcl"
  }

  volume {
    source      = "images.volume.shipyard.run"
    destination = "/cache"
    type        = "volume"
  }
  
  volume {
    source      = "."
    destination = "/test2/${env(resources.template.consul_config.name)}"
  }
}
