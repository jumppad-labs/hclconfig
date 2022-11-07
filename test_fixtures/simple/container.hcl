variable "cpu_resources" {
  default = 2048
}
network "onprem" {
  subnet = "10.6.0.0/16"
}

template "consul_config" {
  disabled = false

  source = <<-EOF
    data_dir = "#{{ .Vars.data_dir }}"
    log_level = "DEBUG"
    
    datacenter = "dc1"
    primary_datacenter = "dc1"
    
    server = true
    
    bootstrap_expect = 1
    ui = true
  EOF

  append_file = true

  destination = "./consul.hcl"

  vars = {
    data_dir = "/tmp"
  }
}

template "consul_config_update" {
  disabled = false

  source = <<-EOF
    # Additional
  EOF

  append_file = resources.template.consul_config.append_file

  destination = "./consul.hcl"

  vars = {
    data_dir = "/tmp"
  }
}

container "base" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resources.network.onprem.name
    ip_address = "10.6.0.200"
  }

  dns = ["a","b","c"]

  resources {
    memory = 1024
    cpu_pin = [1]
  }
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
    destination = "/test"
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
}
