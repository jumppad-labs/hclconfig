variable "cpu_resources" {
  default = 2048
}

resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}

resource "template" "consul_config" {
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

resource "template" "consul_config_update" {
  disabled = false

  source = <<-EOF
    # Additional
  EOF

  append_file = resource.template.consul_config.append_file

  destination = "./consul.hcl"

  vars = {
    data_dir = "/tmp"
  }
}

resource "container" "base" {
  // ensure that arrays can also have interpolation
  entrypoint = [
    resource.network.onprem.id
  ]

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    id         = 1
    name       = resource.network.onprem.name
    ip_address = "10.6.0.200"
  }

  network {
    id         = 2
    name       = resource.network.onprem.name
    ip_address = "10.6.0.201"
  }

  dns = ["a", "b", "c"]

  resources {
    memory  = 1024
    cpu_pin = [1]
    user    = "nic"
  }
}

resource "container" "consul" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = resource.network.onprem.name
    ip_address = "10.6.0.200"
    id         = resource.container.base.network[0].id
  }

  network {
    name       = resource.container.base.network[1].name
    ip_address = "10.7.0.201"
  }

  dns = resource.container.base.dns

  resources {
    # Max CPU to consume, 1024 is one core, default unlimited
    cpu = variable.cpu_resources
    # Pin container to specified CPU cores, default all cores
    cpu_pin = resource.container.base.resources.cpu_pin
    # max memory in MB to consume, default unlimited
    memory = resource.container.base.resources.memory
    user   = resource.container.base.resources.user
  }

  volume {
    source      = "."
    destination = "/test/${resource.template.consul_config.destination}"
  }


  volume {
    source      = resource.template.consul_config.destination
    destination = "/config/config.hcl"
  }

  volume {
    source      = "images.volume.shipyard.run"
    destination = "/cache"
    type        = "volume"
  }

  volume {
    source      = "."
    destination = "/test2/${env(resource.template.consul_config.name)}"
  }
}

output "ip_address_1" {
  value = resource.container.consul.network.0.ip_address
}

output "ip_address_2" {
  value = resource.container.consul.network.1.ip_address
}

output "ip_addresses" {
  value = [
    resource.container.consul.network.0.ip_address,
    resource.container.consul.network.1.ip_address,
    12
  ]
}