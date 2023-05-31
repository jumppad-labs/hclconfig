variable "default_cpu" {
  default = "512"
}

resource "container" "base" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network {
    name       = "testing"
    ip_address = "10.6.0.200"
  }

  dns = ["a", "b", "c"]

  resources {
    memory  = 1024
    cpu_pin = [1]
    cpu     = 4096
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
  variables = {
    cpu_resources = variable.default_cpu
  }
}

module "consul_3" {
  // all resources in this module will only be created after all the 
  // resources in 'consul_1' have been created.
  depends_on = ["module.consul_1"]
  source     = "../single"
}

// returns a simple type from the ouput of the module
output "module1_container_resources_cpu" {
  value = module.consul_1.output.container_resources_cpu
}

// returns a simple type from the ouput of the module
output "module2_container_resources_cpu" {
  value = module.consul_2.output.container_resources_cpu
}

// returns a simple type from the ouput of the module
output "module3_container_resources_cpu" {
  value = module.consul_3.output.container_resources_cpu
}

// returns an element using a numeric index from a list 
// returned from the output
output "module1_from_list_1" {
  value = element(module.consul_1.output.combined_list, 0)
}

//// returns an element using a numeric index from a list 
//// returned from the output
output "module1_from_list_2" {
  value = element(module.consul_1.output.combined_list, 1)
}

// returns an element using a numeric index from a list 
// returned from the output
output "module1_from_list_3" {
  value = module.consul_1.output.combined_list.0
}

// returns an element using a string index from a map
// returned from the output
output "module1_from_map_1" {
  value = element(module.consul_1.output.combined_map, "name")
}

// returns an element using a string index from a map
// returned from the output
output "module1_from_map_2" {
  value = element(module.consul_1.output.combined_map, "cpu")
}

output "module1_from_map_3" {
  value = module.consul_1.output.combined_map.name
}