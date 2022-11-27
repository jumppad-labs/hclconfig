container "base" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = "testing"
    ip_address = "10.6.0.200"
  }

  dns = ["a","b","c"]

  resources {
    memory = 1024
    cpu_pin = [1]
    cpu = 4096
  }
}

module "consul_1" {
  source = "../single"
  variables = {
    cpu_resources = resource.container.base.resources.cpu
  }
}

module "consul_2" {
  source = "../single"
}

output "module1_container_resources_cpu" {
  value = module.consul_1.output.container_resources_cpu
}

output "module2_container_resources_cpu" {
  value = module.consul_2.output.container_resources_cpu
}
